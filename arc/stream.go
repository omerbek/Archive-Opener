package arc

import (
	"archive/tar"
	"bufio"
	"compress/bzip2"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/klauspost/compress/zstd"
	"github.com/ulikunitz/xz"
)

// streamArchive covers .tar, .tar.gz/.tgz, .tar.bz2, .tar.xz, .tar.zst and
// single compressed files (.gz, .bz2, .xz, .zst).
type streamArchive struct {
	path    string
	comp    kind // kTAR when uncompressed
	isTar   bool
	format  string
	entries []*Entry
	single  string // name of the single decompressed file
}

func readHead(path string, n int) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b := make([]byte, n)
	m, err := io.ReadFull(f, b)
	if err == io.ErrUnexpectedEOF {
		err = nil
	}
	return b[:m], err
}

func decompress(k kind, r io.Reader) (io.Reader, func(), error) {
	switch k {
	case kGZ:
		g, err := gzip.NewReader(r)
		if err != nil {
			return nil, nil, err
		}
		return g, func() { g.Close() }, nil
	case kBZ2:
		return bzip2.NewReader(r), func() {}, nil
	case kXZ:
		x, err := xz.NewReader(r)
		if err != nil {
			return nil, nil, err
		}
		return x, func() {}, nil
	case kZSTD:
		z, err := zstd.NewReader(r)
		if err != nil {
			return nil, nil, err
		}
		return z, z.Close, nil
	}
	return r, func() {}, nil
}

var compNames = map[kind]string{kGZ: "GZ", kBZ2: "BZ2", kXZ: "XZ", kZSTD: "ZST"}

func (a *streamArchive) open() (*os.File, io.Reader, func(), error) {
	f, err := os.Open(a.path)
	if err != nil {
		return nil, nil, nil, err
	}
	r, closeFn, err := decompress(a.comp, bufio.NewReaderSize(f, 1<<16))
	if err != nil {
		f.Close()
		return nil, nil, nil, err
	}
	return f, r, closeFn, nil
}

func openStream(path string, k kind) (Archive, error) {
	a := &streamArchive{path: path, comp: k}
	f, r, closeFn, err := a.open()
	if err != nil {
		return nil, err
	}
	defer f.Close()
	defer closeFn()

	br := bufio.NewReaderSize(r, 1024)
	peek, _ := br.Peek(512)
	a.isTar = k == kTAR || (len(peek) >= 262 && string(peek[257:262]) == "ustar") || looksLikeOldTar(peek)

	if !a.isTar {
		// Single compressed file.
		st, _ := f.Stat()
		name := filepath.Base(path)
		for _, suf := range []string{".gz", ".gzip", ".bz2", ".xz", ".zst", ".zstd"} {
			if strings.HasSuffix(strings.ToLower(name), suf) {
				name = name[:len(name)-len(suf)]
				break
			}
		}
		var mod time.Time
		if g, ok := r.(*gzip.Reader); ok {
			if g.Name != "" {
				name = filepath.Base(latin1ToUTF8(g.Name))
			}
			mod = g.ModTime
		}
		if mod.IsZero() && st != nil {
			mod = st.ModTime()
		}
		size, err := io.Copy(io.Discard, br)
		if err != nil {
			return nil, err
		}
		a.single = name
		a.format = compNames[k]
		packed := int64(-1)
		if st != nil {
			packed = st.Size()
		}
		a.entries = []*Entry{{Path: name, Size: size, Packed: packed, Modified: mod}}
		return a, nil
	}

	a.format = "TAR"
	if k != kTAR {
		a.format = "TAR." + compNames[k]
	}
	tr := tar.NewReader(br)
	for i := 0; ; i++ {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			if len(a.entries) > 0 {
				break // tolerate trailing garbage
			}
			return nil, err
		}
		if h.Typeflag == tar.TypeXGlobalHeader {
			i--
			continue
		}
		name := strings.TrimLeft(strings.TrimPrefix(h.Name, "./"), "/")
		if name == "" || name == "." {
			i--
			continue
		}
		e := &Entry{
			Path:     name,
			Size:     h.Size,
			Packed:   -1,
			Modified: h.ModTime,
			IsDir:    h.Typeflag == tar.TypeDir,
			IsLink:   h.Typeflag == tar.TypeSymlink || h.Typeflag == tar.TypeLink,
			index:    i,
		}
		if !e.IsDir && !e.IsLink && h.Typeflag != tar.TypeReg && h.Typeflag != tar.TypeRegA && h.Typeflag != tar.TypeGNUSparse {
			e.IsLink = true // devices, fifos: skip on extract
		}
		if e.IsDir && !strings.HasSuffix(e.Path, "/") {
			e.Path += "/"
		}
		a.entries = append(a.entries, e)
	}
	return a, nil
}

func looksLikeOldTar(b []byte) bool {
	// Pre-POSIX tar: checksum field at 148 holds octal digits.
	if len(b) < 512 {
		return false
	}
	for _, c := range b[148:154] {
		if c != ' ' && (c < '0' || c > '7') {
			return false
		}
	}
	return b[0] != 0
}

func (a *streamArchive) Format() string    { return a.format }
func (a *streamArchive) Entries() []*Entry { return a.entries }
func (a *streamArchive) Close() error      { return nil }

func (a *streamArchive) Walk(want func(*Entry) bool, fn func(*Entry, io.Reader) error) error {
	f, r, closeFn, err := a.open()
	if err != nil {
		return err
	}
	defer f.Close()
	defer closeFn()

	if !a.isTar {
		e := a.entries[0]
		if want != nil && !want(e) {
			return nil
		}
		return fn(e, r)
	}

	remaining := 0
	for _, e := range a.entries {
		if want == nil || want(e) {
			remaining++
		}
	}
	if remaining == 0 {
		return nil
	}
	tr := tar.NewReader(r)
	i := 0
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		name := strings.TrimLeft(strings.TrimPrefix(h.Name, "./"), "/")
		if h.Typeflag == tar.TypeXGlobalHeader || name == "" || name == "." {
			continue
		}
		if i >= len(a.entries) {
			return nil
		}
		e := a.entries[i]
		i++
		if want != nil && !want(e) {
			continue
		}
		var rd io.Reader = tr
		if e.IsDir || e.IsLink {
			rd = strings.NewReader("")
		}
		if err := fn(e, rd); err != nil {
			return err
		}
		remaining--
		if remaining == 0 {
			return nil
		}
	}
}

// latin1ToUTF8 undoes Go's Latin-1 decoding of gzip names when the bytes
// were really UTF-8, which is what most tools write.
func latin1ToUTF8(s string) string {
	b := make([]byte, 0, len(s))
	for _, r := range s {
		if r > 0xff {
			return s
		}
		b = append(b, byte(r))
	}
	if utf8.Valid(b) {
		return string(b)
	}
	return s
}
