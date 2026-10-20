//go:build windows

package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// fmtSize prints sizes the way Turkish Windows does: 1,5 MB.
func fmtSize(b int64) string {
	if b < 0 {
		return ""
	}
	if b < 1024 {
		return fmt.Sprintf("%d B", b)
	}
	units := []string{"KB", "MB", "GB", "TB"}
	v := float64(b) / 1024
	i := 0
	for v >= 1024 && i < len(units)-1 {
		v /= 1024
		i++
	}
	s := fmt.Sprintf("%.1f", v)
	if v >= 100 {
		s = fmt.Sprintf("%.0f", v)
	}
	return strings.Replace(s, ".", ",", 1) + " " + units[i]
}

func fmtDate(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Local().Format("02.01.2006 15:04")
}

// archiveBaseName strips archive extensions: "film.part1.rar" -> "film",
// "yedek.tar.gz" -> "yedek".
func archiveBaseName(path string) string {
	name := filepath.Base(path)
	lower := strings.ToLower(name)
	for _, suf := range []string{".tar.gz", ".tar.bz2", ".tar.xz", ".tar.zst", ".tgz", ".tbz2", ".txz"} {
		if strings.HasSuffix(lower, suf) {
			return name[:len(name)-len(suf)]
		}
	}
	name = strings.TrimSuffix(name, filepath.Ext(name))
	lower = strings.ToLower(name)
	if i := strings.LastIndex(lower, ".part"); i > 0 && isDigits(lower[i+5:]) {
		name = name[:i]
	}
	if name == "" {
		name = "arsiv"
	}
	return name
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// typeName gives a readable type for the Type column.
func typeName(name string, dir bool) string {
	if dir {
		return "Dosya klasörü"
	}
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(name)), ".")
	if ext == "" {
		return "Dosya"
	}
	return strings.ToUpper(ext) + " dosyası"
}
