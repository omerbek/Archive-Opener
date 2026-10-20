package arc

import (
	"bytes"
	"encoding/binary"
	"io"
	"os"
	"reflect"
	"strings"

	"github.com/bodgit/sevenzip"
)

var aesCoderID = []byte{0x06, 0xf1, 0x07, 0x01}

type sevenArchive struct {
	path     string
	password string
	entries  []*Entry
}

func open7Z(path, password string) (Archive, error) {
	rc, err := sevenzip.OpenReaderWithPassword(path, password)
	if err != nil {
		if password == "" && headerEncrypted7z(path) {
			return nil, ErrNeedPassword
		}
		err = classify(err)
		if password != "" && err != ErrNeedPassword {
			return nil, ErrBadPassword
		}
		return nil, err
	}
	defer rc.Close()
	encFolders := encryptedFolders(&rc.Reader)
	a := &sevenArchive{path: path, password: password}
	for i, f := range rc.File {
		name := strings.TrimLeft(strings.ReplaceAll(f.Name, "\\", "/"), "/")
		info := f.FileInfo()
		e := &Entry{
			Path:     name,
			Size:     int64(f.UncompressedSize),
			Packed:   -1,
			Modified: f.Modified,
			IsDir:    info.IsDir(),
			IsLink:   info.Mode()&os.ModeSymlink != 0,
			index:    i,
		}
		if fi := fileFolder(f); fi >= 0 && fi < len(encFolders) && !e.IsDir && e.Size > 0 {
			e.Encrypted = encFolders[fi]
		}
		if e.IsDir && !strings.HasSuffix(e.Path, "/") {
			e.Path += "/"
		}
		a.entries = append(a.entries, e)
	}
	return a, nil
}

// headerEncrypted7z reports whether the archive's file list itself is
// encrypted, by looking for the AES coder in the (unencrypted) header
// descriptor.
func headerEncrypted7z(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	var sig [32]byte
	if _, err := io.ReadFull(f, sig[:]); err != nil {
		return false
	}
	off := binary.LittleEndian.Uint64(sig[12:])
	size := binary.LittleEndian.Uint64(sig[20:])
	if size == 0 || size > 1<<20 {
		return false
	}
	hdr := make([]byte, size)
	if _, err := f.ReadAt(hdr, 32+int64(off)); err != nil {
		return false
	}
	return hdr[0] == 0x17 && bytes.Contains(hdr, aesCoderID)
}

// encryptedFolders reads the coder list the library keeps unexported, to
// learn which compressed streams use AES. The library version is pinned,
// and any surprise in the layout just yields "unknown" (false).
func encryptedFolders(r *sevenzip.Reader) (out []bool) {
	defer func() {
		if recover() != nil {
			out = nil
		}
	}()
	si := reflect.ValueOf(r).Elem().FieldByName("si")
	if !si.IsValid() || si.IsNil() {
		return nil
	}
	ui := si.Elem().FieldByName("unpackInfo")
	if !ui.IsValid() || ui.IsNil() {
		return nil
	}
	folders := ui.Elem().FieldByName("folder")
	for i := 0; i < folders.Len(); i++ {
		coders := folders.Index(i).Elem().FieldByName("coder")
		enc := false
		for j := 0; j < coders.Len(); j++ {
			if bytes.Equal(coders.Index(j).Elem().FieldByName("id").Bytes(), aesCoderID) {
				enc = true
			}
		}
		out = append(out, enc)
	}
	return out
}

func fileFolder(f *sevenzip.File) (idx int) {
	defer func() {
		if recover() != nil {
			idx = -1
		}
	}()
	return int(reflect.ValueOf(f).Elem().FieldByName("folder").Int())
}

func (a *sevenArchive) Format() string    { return "7Z" }
func (a *sevenArchive) Entries() []*Entry { return a.entries }
func (a *sevenArchive) Close() error      { return nil }

func (a *sevenArchive) Walk(want func(*Entry) bool, fn func(*Entry, io.Reader) error) error {
	// A fresh reader per walk: the library's stream cache does not always
	// survive a second pass over encrypted data.
	rc, err := sevenzip.OpenReaderWithPassword(a.path, a.password)
	if err != nil {
		return classify(err)
	}
	defer rc.Close()
	for i, e := range a.entries {
		if want != nil && !want(e) {
			continue
		}
		if e.IsDir || e.IsLink {
			if err := fn(e, strings.NewReader("")); err != nil {
				return err
			}
			continue
		}
		if e.Encrypted && a.password == "" {
			return ErrNeedPassword
		}
		r, err := rc.File[i].Open()
		if err != nil {
			return a.mapErr(e, err)
		}
		err = fn(e, &sevenReader{r: r, a: a, e: e})
		r.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func (a *sevenArchive) mapErr(e *Entry, err error) error {
	err = classify(err)
	if err == ErrNeedPassword || err == ErrBadPassword || err == ErrStop {
		return err
	}
	if e.Encrypted {
		if a.password == "" {
			return ErrNeedPassword
		}
		return ErrBadPassword
	}
	return err
}

type sevenReader struct {
	r io.Reader
	a *sevenArchive
	e *Entry
}

func (s *sevenReader) Read(p []byte) (int, error) {
	n, err := s.r.Read(p)
	if err != nil && err != io.EOF {
		err = s.a.mapErr(s.e, err)
	}
	return n, err
}
