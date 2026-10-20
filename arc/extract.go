package arc

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Conflict is the answer to "a file with this name already exists".
type Conflict int

const (
	Overwrite Conflict = iota
	Skip
	Cancel
)

// ExtractOptions controls Extract.
type ExtractOptions struct {
	Dest string
	// Want selects entries; nil means all.
	Want func(*Entry) bool
	// StripPrefix is removed from the start of each entry path, so files
	// picked from inside a folder land directly in Dest.
	StripPrefix string
	// OnConflict is asked when a target file exists; nil means Overwrite.
	OnConflict func(target string, e *Entry) Conflict
	// Progress reports bytes written; returning false cancels.
	Progress func(done, total int64, name string) bool
}

// Result summarizes an extraction.
type Result struct {
	Files   int
	Skipped int
	Unsafe  []string // entries refused because of their path or type
	Written []string // paths of files written
}

// ErrCanceled is returned when the user stops an extraction.
var ErrCanceled = errors.New("işlem iptal edildi")

var reservedNames = map[string]bool{
	"CON": true, "PRN": true, "AUX": true, "NUL": true,
	"COM1": true, "COM2": true, "COM3": true, "COM4": true, "COM5": true, "COM6": true, "COM7": true, "COM8": true, "COM9": true,
	"LPT1": true, "LPT2": true, "LPT3": true, "LPT4": true, "LPT5": true, "LPT6": true, "LPT7": true, "LPT8": true, "LPT9": true,
}

// SafeRel turns an archive path into a relative OS path that cannot leave
// the destination folder. ok is false when the path is unusable.
func SafeRel(p string) (rel string, ok bool) {
	p = strings.ReplaceAll(p, "\\", "/")
	var parts []string
	for _, part := range strings.Split(p, "/") {
		switch part {
		case "", ".":
			continue
		case "..":
			return "", false
		}
		part = cleanName(part)
		if part == "" {
			continue
		}
		parts = append(parts, part)
	}
	if len(parts) == 0 {
		return "", false
	}
	return filepath.Join(parts...), true
}

func cleanName(s string) string {
	b := []rune(s)
	for i, r := range b {
		if r < 32 || strings.ContainsRune(`<>:"|?*`, r) {
			b[i] = '_'
		}
	}
	s = strings.TrimRight(string(b), ". ")
	base := strings.ToUpper(s)
	if i := strings.IndexByte(base, '.'); i >= 0 {
		base = base[:i]
	}
	if reservedNames[base] {
		s = "_" + s
	}
	return s
}

// Extract writes the selected entries below o.Dest.
func Extract(a Archive, o ExtractOptions) (Result, error) {
	var res Result
	want := func(e *Entry) bool {
		if o.StripPrefix != "" && !strings.HasPrefix(e.Path, o.StripPrefix) {
			return false
		}
		return o.Want == nil || o.Want(e)
	}
	total := TotalSize(a, want)
	var done int64
	type dirTime struct {
		path string
		t    time.Time
	}
	var dirTimes []dirTime

	if err := os.MkdirAll(o.Dest, 0o755); err != nil {
		return res, err
	}

	err := a.Walk(want, func(e *Entry, r io.Reader) error {
		rel, ok := SafeRel(strings.TrimPrefix(e.Path, o.StripPrefix))
		if e.IsLink || !ok {
			if !e.IsDir || !ok {
				res.Unsafe = append(res.Unsafe, e.Path)
			}
			return nil
		}
		target := filepath.Join(o.Dest, rel)
		if e.IsDir {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			if !e.Modified.IsZero() {
				dirTimes = append(dirTimes, dirTime{target, e.Modified})
			}
			return nil
		}
		if o.Progress != nil && !o.Progress(done, total, e.Path) {
			return ErrCanceled
		}
		if st, err := os.Lstat(target); err == nil {
			if st.IsDir() {
				res.Unsafe = append(res.Unsafe, e.Path)
				return nil
			}
			c := Overwrite
			if o.OnConflict != nil {
				c = o.OnConflict(target, e)
			}
			switch c {
			case Skip:
				res.Skipped++
				done += e.Size
				return nil
			case Cancel:
				return ErrCanceled
			}
			os.Chmod(target, 0o644) // allow overwriting read-only files
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		out, err := os.Create(target)
		if err != nil {
			return err
		}
		pw := &progressWriter{fn: func(n int64) bool {
			done += n
			return o.Progress == nil || o.Progress(done, total, e.Path)
		}}
		_, err = io.Copy(io.MultiWriter(out, pw), r)
		cerr := out.Close()
		if err == nil {
			err = cerr
		}
		if err != nil {
			os.Remove(target)
			if errors.Is(err, ErrStop) {
				return ErrCanceled
			}
			return err
		}
		if !e.Modified.IsZero() {
			os.Chtimes(target, e.Modified, e.Modified)
		}
		res.Files++
		res.Written = append(res.Written, target)
		return nil
	})
	for i := len(dirTimes) - 1; i >= 0; i-- {
		os.Chtimes(dirTimes[i].path, dirTimes[i].t, dirTimes[i].t)
	}
	if o.Progress != nil && err == nil {
		o.Progress(total, total, "")
	}
	return res, err
}
