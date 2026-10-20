package arc

import (
	"errors"
	"io"
	"os"
	"strings"

	"github.com/nwaples/rardecode/v2"
)

type rarArchive struct {
	path     string
	password string
	format   string
	entries  []*Entry
}

func rarOpts(password string) []rardecode.Option {
	if password == "" {
		return nil
	}
	return []rardecode.Option{rardecode.Password(password)}
}

func openRAR(path, password string) (Archive, error) {
	files, err := rardecode.List(path, rarOpts(password)...)
	if err != nil {
		err = classify(err)
		if password != "" && err != ErrNeedPassword {
			// Encrypted headers decrypted with the wrong key read as garbage.
			return nil, ErrBadPassword
		}
		return nil, err
	}
	a := &rarArchive{path: path, password: password, format: "RAR"}
	if isRAR5(path) {
		a.format = "RAR5"
	}
	for i, f := range files {
		name := strings.TrimPrefix(strings.ReplaceAll(f.Name, "\\", "/"), "/")
		e := &Entry{
			Path:      name,
			Size:      f.UnPackedSize,
			Packed:    f.PackedSize,
			Modified:  f.ModificationTime,
			IsDir:     f.IsDir,
			Encrypted: f.Encrypted,
			IsLink:    f.LinkType != 0 || f.Mode()&os.ModeSymlink != 0,
			index:     i,
		}
		if e.IsDir && !strings.HasSuffix(e.Path, "/") {
			e.Path += "/"
		}
		a.entries = append(a.entries, e)
	}
	return a, nil
}

func isRAR5(path string) bool {
	head, err := readHead(path, 8)
	return err == nil && len(head) >= 8 && string(head[:8]) == "Rar!\x1a\x07\x01\x00"
}

func (a *rarArchive) Format() string     { return a.format }
func (a *rarArchive) Entries() []*Entry  { return a.entries }
func (a *rarArchive) Close() error       { return nil }

func (a *rarArchive) Walk(want func(*Entry) bool, fn func(*Entry, io.Reader) error) error {
	// Count how many entries are wanted so the walk can stop early.
	remaining := 0
	for _, e := range a.entries {
		if want == nil || want(e) {
			remaining++
		}
	}
	if remaining == 0 {
		return nil
	}
	rc, err := rardecode.OpenReader(a.path, rarOpts(a.password)...)
	if err != nil {
		return classify(err)
	}
	defer rc.Close()
	for i := 0; ; i++ {
		h, err := rc.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return classify(err)
		}
		if i >= len(a.entries) {
			return nil
		}
		e := a.entries[i]
		if want != nil && !want(e) {
			continue
		}
		var r io.Reader = rc
		if e.IsDir || e.IsLink {
			r = strings.NewReader("")
		} else if h.Encrypted {
			r = &rarPassReader{r: rc, hasPass: a.password != ""}
		}
		if err := fn(e, r); err != nil {
			return err
		}
		remaining--
		if remaining == 0 {
			return nil
		}
	}
}

// rarPassReader turns the checksum errors a wrong password causes in old
// RAR archives into ErrBadPassword.
type rarPassReader struct {
	r       io.Reader
	hasPass bool
}

func (p *rarPassReader) Read(b []byte) (int, error) {
	n, err := p.r.Read(b)
	if err != nil && err != io.EOF {
		if !p.hasPass {
			return n, ErrNeedPassword
		}
		if errors.Is(err, rardecode.ErrBadFileChecksum) || errors.Is(err, rardecode.ErrBadPassword) ||
			strings.Contains(err.Error(), "corrupt") || strings.Contains(err.Error(), "huffman") {
			return n, ErrBadPassword
		}
	}
	return n, err
}
