//go:build windows

package main

import (
	_ "embed"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"

	"arsivacici/arc"
)

//go:embed app.ico
var icoData []byte

// appIcon stays an untyped nil when the icon can't be loaded, so walk never
// sees a typed nil *walk.Icon.
var appIcon Property

const appName = "Arşiv Açıcı"
const appVersion = "1.0"

// loadAppIcon writes the embedded icon to a temporary file (walk loads
// icon sizes from the file lazily, so it must stay until exit) and returns
// the folder to delete afterwards.
func loadAppIcon() string {
	dir, err := os.MkdirTemp("", "ArsivAcici-ico-")
	if err != nil {
		return ""
	}
	p := filepath.Join(dir, "app.ico")
	if os.WriteFile(p, icoData, 0o644) == nil {
		if ic, err := walk.NewIconFromFile(p); err == nil && ic != nil {
			appIcon = ic
		}
	}
	return dir
}

func main() {
	walk.App().SetOrganizationName("ArsivAcici")
	walk.App().SetProductName("ArsivAcici")
	icoDir := loadAppIcon()

	code := 0
	args := os.Args[1:]
	if len(args) == 2 && (args[0] == "-x" || args[0] == "-xh") {
		code = quickExtract(args[1], args[0] == "-xh")
	} else {
		a := &app{}
		start := ""
		if len(args) > 0 {
			start = args[0]
		}
		a.run(start)
	}
	if icoDir != "" {
		os.RemoveAll(icoDir)
	}
	os.Exit(code)
}

/* ---------------- right-click "extract" mode, no main window ---------------- */

func quickExtract(path string, here bool) int {
	dest := filepath.Dir(path)
	if !here {
		dest = filepath.Join(dest, archiveBaseName(path))
	}
	pw := ""
	wrong := false
	for {
		a, err := arc.Open(path, pw)
		if err == nil {
			if pw == "" && anyEncrypted(a.Entries(), nil) {
				a.Close()
				p, ok := askPassword(nil, filepath.Base(path), false)
				if !ok {
					return 1
				}
				pw = p
				continue
			}
			err = extractWithJob(nil, a, dest, arc.ExtractOptions{}, 0)
			a.Close()
			if errors.Is(err, arc.ErrBadPassword) {
				wrong = true
				p, ok := askPassword(nil, filepath.Base(path), true)
				if !ok {
					return 1
				}
				pw = p
				continue
			}
			if err != nil && !errors.Is(err, arc.ErrCanceled) {
				walk.MsgBox(nil, appName, "Çıkarma tamamlanamadı:\n"+friendly(err), walk.MsgBoxIconError)
				return 1
			}
			return 0
		}
		if errors.Is(err, arc.ErrNeedPassword) || errors.Is(err, arc.ErrBadPassword) {
			p, ok := askPassword(nil, filepath.Base(path), wrong || errors.Is(err, arc.ErrBadPassword))
			if !ok {
				return 1
			}
			pw = p
			continue
		}
		walk.MsgBox(nil, appName, fmt.Sprintf("\"%s\" açılamadı:\n%s", filepath.Base(path), friendly(err)), walk.MsgBoxIconError)
		return 1
	}
}

func anyEncrypted(entries []*arc.Entry, want func(*arc.Entry) bool) bool {
	for _, e := range entries {
		if e.Encrypted && (want == nil || want(e)) {
			return true
		}
	}
	return false
}

func friendly(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, arc.ErrUnknownFormat):
		return "Bu dosya tanınan bir arşiv değil ya da bozuk."
	case errors.Is(err, arc.ErrBadPassword):
		return "Şifre yanlış."
	case errors.Is(err, arc.ErrNeedPassword):
		return "Bu arşiv şifreli."
	case errors.Is(err, os.ErrNotExist):
		return "Dosya bulunamadı."
	case errors.Is(err, os.ErrPermission):
		return "Bu klasöre yazma izniniz yok. Başka bir hedef klasör seçin."
	case errors.Is(err, io.ErrUnexpectedEOF):
		return "Arşiv eksik ya da bozuk (dosyanın sonu yarım). Çok parçalı bir arşivse tüm parçaların aynı klasörde olduğundan emin olun."
	}
	s := err.Error()
	if strings.Contains(s, "unsupported decoder") {
		return "Bu arşiv çok eski bir RAR sürümüyle (1.5) oluşturulmuş; desteklenmiyor."
	}
	if strings.Contains(s, "multi-volume") || strings.Contains(s, "volume") {
		return "Çok parçalı arşivin bir parçası eksik. Tüm parçaları aynı klasöre koyun ve ilk parçayı (part1) açın.\n\n(" + s + ")"
	}
	if strings.Contains(s, "checksum") || strings.Contains(s, "CRC") {
		return "Arşivde bozuk veri var (sağlama toplamı tutmadı).\n\n(" + s + ")"
	}
	return s
}

// extractWithJob runs arc.Extract behind a progress window. conflict is
// 0 ask, 1 overwrite, 2 skip.
func extractWithJob(owner walk.Form, a arc.Archive, dest string, o arc.ExtractOptions, conflict int) error {
	o.Dest = dest
	var res arc.Result
	err := runJob(owner, "Çıkarılıyor — "+appName, func(j *job) error {
		policy := conflict
		o.Progress = j.update
		o.OnConflict = func(target string, e *arc.Entry) arc.Conflict {
			switch policy {
			case 1:
				return arc.Overwrite
			case 2:
				return arc.Skip
			}
			switch j.ask(func(w walk.Form) int { return askConflict(w, target, e) }) {
			case cOverwrite:
				return arc.Overwrite
			case cOverwriteAll:
				policy = 1
				return arc.Overwrite
			case cSkip:
				return arc.Skip
			case cSkipAll:
				policy = 2
				return arc.Skip
			}
			return arc.Cancel
		}
		var err error
		res, err = arc.Extract(a, o)
		return err
	})
	if err == nil && len(res.Unsafe) > 0 {
		list := res.Unsafe
		more := ""
		if len(list) > 8 {
			more = fmt.Sprintf("\n… ve %d tane daha", len(list)-8)
			list = list[:8]
		}
		walk.MsgBox(owner, appName,
			fmt.Sprintf("%d dosya çıkarıldı.\n\nGüvenlik nedeniyle atlananlar (kısayol/bağlantı ya da hedef klasörün dışına yazmaya çalışan yollar):\n%s%s",
				res.Files, strings.Join(list, "\n"), more), walk.MsgBoxIconInformation)
	}
	return err
}
