// Package arc reads archives (RAR, ZIP, 7z, TAR and single-file
// compressed streams) behind one small interface and extracts them safely.
package arc

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// Entry is one file or folder inside an archive.
type Entry struct {
	Path      string // always uses '/' as separator, no leading slash
	Size      int64
	Packed    int64 // compressed size, -1 when unknown
	Modified  time.Time
	IsDir     bool
	Encrypted bool
	IsLink    bool // symbolic or hard link; never extracted
	index     int
}

// Name returns the last element of the entry path.
func (e *Entry) Name() string {
	p := strings.TrimSuffix(e.Path, "/")
	if i := strings.LastIndexByte(p, '/'); i >= 0 {
		return p[i+1:]
	}
	return p
}

// Archive is an opened archive.
type Archive interface {
	// Format is a short display name such as "RAR5" or "ZIP".
	Format() string
	// Entries lists every entry in archive order.
	Entries() []*Entry
	// Walk streams the entries for which want returns true, in archive
	// order, calling fn with a reader for the entry contents. Directories
	// and links are passed with an empty reader.
	Walk(want func(*Entry) bool, fn func(*Entry, io.Reader) error) error
	Close() error
}

var (
	// ErrNeedPassword means the archive or some of its files are encrypted
	// and no password was given.
	ErrNeedPassword = errors.New("bu arşiv şifreli")
	// ErrBadPassword means the given password is wrong.
	ErrBadPassword = errors.New("şifre yanlış")
	// ErrUnknownFormat means the file is not an archive this package reads.
	ErrUnknownFormat = errors.New("bu dosya tanınan bir arşiv biçiminde değil")
	// ErrStop can be returned from a Walk callback to stop early without error.
	ErrStop = errors.New("stop")
)

type kind int

const (
	kUnknown kind = iota
	kRAR
	kZIP
	k7Z
	kTAR
	kGZ
	kBZ2
	kXZ
	kZSTD
)

func sniff(head []byte) kind {
	has := func(sig []byte, off int) bool {
		return len(head) >= off+len(sig) && bytes.Equal(head[off:off+len(sig)], sig)
	}
	switch {
	case has([]byte("Rar!\x1a\x07"), 0):
		return kRAR
	case has([]byte("PK\x03\x04"), 0), has([]byte("PK\x05\x06"), 0), has([]byte("PK\x07\x08"), 0):
		return kZIP
	case has([]byte{'7', 'z', 0xbc, 0xaf, 0x27, 0x1c}, 0):
		return k7Z
	case has([]byte{0x1f, 0x8b}, 0):
		return kGZ
	case has([]byte("BZh"), 0):
		return kBZ2
	case has([]byte{0xfd, '7', 'z', 'X', 'Z', 0}, 0):
		return kXZ
	case has([]byte{0x28, 0xb5, 0x2f, 0xfd}, 0):
		return kZSTD
	case has([]byte("ustar"), 257):
		return kTAR
	}
	return kUnknown
}

// Open opens the archive at path. password may be empty.
func Open(path, password string) (Archive, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	head := make([]byte, 512)
	n, _ := io.ReadFull(f, head)
	head = head[:n]
	k := sniff(head)
	if k == kUnknown {
		// Self-extracting archives and ZIPs with a prefix: look further in.
		k = sniffDeep(f)
	}
	f.Close()

	switch k {
	case kRAR:
		return openRAR(path, password)
	case kZIP:
		return openZIP(path, password)
	case k7Z:
		return open7Z(path, password)
	case kTAR, kGZ, kBZ2, kXZ, kZSTD:
		return openStream(path, k)
	}
	if strings.HasSuffix(strings.ToLower(path), ".tar") {
		return openStream(path, kTAR)
	}
	return nil, ErrUnknownFormat
}

// sniffDeep looks for a RAR or ZIP signature in the first few megabytes,
// which finds self-extracting .exe archives.
func sniffDeep(f *os.File) kind {
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return kUnknown
	}
	r := bufio.NewReaderSize(io.LimitReader(f, 8<<20), 1<<16)
	data, _ := io.ReadAll(r)
	if bytes.Contains(data, []byte("Rar!\x1a\x07")) {
		return kRAR
	}
	if bytes.Contains(data, []byte("PK\x05\x06")) || bytes.Contains(data, []byte("PK\x03\x04")) {
		return kZIP
	}
	if bytes.Contains(data, []byte{'7', 'z', 0xbc, 0xaf, 0x27, 0x1c}) {
		return k7Z
	}
	return kUnknown
}

// TotalSize returns the summed size of the entries selected by want.
func TotalSize(a Archive, want func(*Entry) bool) int64 {
	var t int64
	for _, e := range a.Entries() {
		if !e.IsDir && (want == nil || want(e)) {
			t += e.Size
		}
	}
	return t
}

// Test reads every file in the archive and reports the first damaged one.
func Test(a Archive, progress func(done int64, name string) bool) error {
	var done int64
	return a.Walk(nil, func(e *Entry, r io.Reader) error {
		if e.IsDir || e.IsLink {
			return nil
		}
		pw := &progressWriter{fn: func(n int64) bool { done += n; return progress == nil || progress(done, e.Path) }}
		if _, err := io.Copy(pw, r); err != nil {
			if errors.Is(err, ErrStop) {
				return err
			}
			return fmt.Errorf("%s: %w", e.Path, classify(err))
		}
		return nil
	})
}

type progressWriter struct {
	fn func(n int64) bool
}

func (p *progressWriter) Write(b []byte) (int, error) {
	if !p.fn(int64(len(b))) {
		return 0, ErrStop
	}
	return len(b), nil
}

// classify maps library-specific errors to friendlier ones.
func classify(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrNeedPassword) || errors.Is(err, ErrBadPassword) || errors.Is(err, ErrStop) {
		return err
	}
	s := err.Error()
	switch {
	case strings.Contains(s, "password required"), strings.Contains(s, "no password set"):
		return ErrNeedPassword
	case strings.Contains(s, "incorrect password"):
		return ErrBadPassword
	}
	return err
}
