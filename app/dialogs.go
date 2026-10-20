//go:build windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"

	"arsivacici/arc"
)

/* ---------------- password ---------------- */

// askPassword shows a password prompt. ok is false when the user cancels.
func askPassword(owner walk.Form, archiveName string, wrongBefore bool) (string, bool) {
	var dlg *walk.Dialog
	var edit *walk.LineEdit
	var okBtn, cancelBtn *walk.PushButton
	var show *walk.CheckBox
	msg := fmt.Sprintf("\"%s\" şifreli. Şifreyi yazın:", archiveName)
	if wrongBefore {
		msg = "Şifre yanlış. Tekrar deneyin:"
	}
	pw := ""
	err := Dialog{
		AssignTo:      &dlg,
		Title:         "Şifre gerekli",
		Icon:          appIcon,
		DefaultButton: &okBtn,
		CancelButton:  &cancelBtn,
		MinSize:       Size{Width: 380},
		Layout:        VBox{},
		Children: []Widget{
			Label{Text: msg},
			LineEdit{AssignTo: &edit, PasswordMode: true},
			CheckBox{AssignTo: &show, Text: "Şifreyi göster", OnCheckedChanged: func() {
				edit.SetPasswordMode(!show.Checked())
			}},
			Composite{
				Layout: HBox{MarginsZero: true},
				Children: []Widget{
					HSpacer{},
					PushButton{AssignTo: &okBtn, Text: "Tamam", OnClicked: func() {
						pw = edit.Text()
						dlg.Accept()
					}},
					PushButton{AssignTo: &cancelBtn, Text: "İptal", OnClicked: func() { dlg.Cancel() }},
				},
			},
		},
	}.Create(owner)
	if err != nil {
		return "", false
	}
	dlg.Starting().Attach(func() { edit.SetFocus() })
	if dlg.Run() != walk.DlgCmdOK || pw == "" {
		return "", false
	}
	return pw, true
}

/* ---------------- file exists ---------------- */

const (
	cOverwrite = iota + 10
	cOverwriteAll
	cSkip
	cSkipAll
	cCancel
)

func askConflict(owner walk.Form, target string, e *arc.Entry) int {
	var dlg *walk.Dialog
	old := ""
	if st, err := os.Stat(target); err == nil {
		old = fmt.Sprintf("Var olan:  %s, %s", fmtSize(st.Size()), fmtDate(st.ModTime()))
	}
	neu := fmt.Sprintf("Arşivdeki: %s, %s", fmtSize(e.Size), fmtDate(e.Modified))
	btn := func(text string, code int) PushButton {
		return PushButton{Text: text, OnClicked: func() { dlg.Close(code) }}
	}
	err := Dialog{
		AssignTo: &dlg,
		Title:    "Dosya zaten var",
		Icon:     appIcon,
		MinSize:  Size{Width: 480},
		Layout:   VBox{},
		Children: []Widget{
			Label{Text: "Bu dosya hedef klasörde zaten var:"},
			Label{Text: target, Font: Font{Bold: true}},
			Label{Text: old},
			Label{Text: neu},
			VSpacer{Size: 6},
			Composite{
				Layout: Grid{Columns: 2, MarginsZero: true},
				Children: []Widget{
					btn("Üzerine yaz", cOverwrite),
					btn("Tümünün üzerine yaz", cOverwriteAll),
					btn("Atla", cSkip),
					btn("Tümünü atla", cSkipAll),
				},
			},
			Composite{
				Layout:   HBox{MarginsZero: true},
				Children: []Widget{HSpacer{}, btn("Çıkarmayı durdur", cCancel)},
			},
		},
	}.Create(owner)
	if err != nil {
		return cCancel
	}
	r := dlg.Run()
	if r < cOverwrite || r > cCancel {
		return cCancel
	}
	return r
}

/* ---------------- extract options ---------------- */

type extractChoice struct {
	dest       string
	conflict   int // 0 ask, 1 overwrite, 2 skip
	openFolder bool
}

func askExtractOptions(owner walk.Form, defDest string, what string) (extractChoice, bool) {
	var dlg *walk.Dialog
	var dest *walk.LineEdit
	var okBtn, cancelBtn *walk.PushButton
	var rbAsk, rbOver, rbSkip *walk.RadioButton
	var openCB *walk.CheckBox
	var ch extractChoice
	err := Dialog{
		AssignTo:      &dlg,
		Title:         "Çıkarma yolu ve seçenekler",
		Icon:          appIcon,
		DefaultButton: &okBtn,
		CancelButton:  &cancelBtn,
		MinSize:       Size{Width: 560},
		Layout:        VBox{},
		Children: []Widget{
			Label{Text: what},
			Label{Text: "Hedef klasör (yoksa oluşturulur):"},
			Composite{
				Layout: HBox{MarginsZero: true},
				Children: []Widget{
					LineEdit{AssignTo: &dest, Text: defDest},
					PushButton{Text: "Gözat…", OnClicked: func() {
						fd := &walk.FileDialog{Title: "Hedef klasörü seçin", FilePath: dest.Text()}
						if ok, _ := fd.ShowBrowseFolder(dlg); ok {
							dest.SetText(fd.FilePath)
						}
					}},
				},
			},
			GroupBox{
				Title:  "Aynı adlı dosya varsa",
				Layout: VBox{},
				Children: []Widget{
					RadioButton{AssignTo: &rbAsk, Text: "Her seferinde sor"},
					RadioButton{AssignTo: &rbOver, Text: "Üzerine yaz"},
					RadioButton{AssignTo: &rbSkip, Text: "Var olanları atla"},
				},
			},
			CheckBox{AssignTo: &openCB, Text: "Bitince klasörü Explorer'da aç", Checked: true},
			Composite{
				Layout: HBox{MarginsZero: true},
				Children: []Widget{
					HSpacer{},
					PushButton{AssignTo: &okBtn, Text: "Çıkar", OnClicked: func() {
						ch.dest = filepath.Clean(dest.Text())
						switch {
						case rbOver.Checked():
							ch.conflict = 1
						case rbSkip.Checked():
							ch.conflict = 2
						}
						ch.openFolder = openCB.Checked()
						if dest.Text() == "" {
							walk.MsgBox(dlg, "Çıkar", "Bir hedef klasör yazın.", walk.MsgBoxIconWarning)
							return
						}
						dlg.Accept()
					}},
					PushButton{AssignTo: &cancelBtn, Text: "İptal", OnClicked: func() { dlg.Cancel() }},
				},
			},
		},
	}.Create(owner)
	if err != nil {
		return ch, false
	}
	rbAsk.SetChecked(true)
	return ch, dlg.Run() == walk.DlgCmdOK
}

/* ---------------- progress ---------------- */

// job is the handle a long operation uses to report progress and to ask
// the user questions from its goroutine.
type job struct {
	dlg      *walk.Dialog
	label    *walk.Label
	detail   *walk.Label
	bar      *walk.ProgressBar
	canceled atomic.Bool
	mu       sync.Mutex
	last     time.Time
	start    time.Time
}

// update reports progress; it returns false once the user pressed Cancel.
func (j *job) update(done, total int64, name string) bool {
	j.mu.Lock()
	now := time.Now()
	if now.Sub(j.last) < 80*time.Millisecond && done < total {
		j.mu.Unlock()
		return !j.canceled.Load()
	}
	j.last = now
	j.mu.Unlock()
	j.dlg.Synchronize(func() {
		if name != "" {
			j.label.SetText(name)
		}
		pos := 0
		if total > 0 {
			pos = int(done * 1000 / total)
		}
		j.bar.SetValue(pos)
		speed := ""
		if el := time.Since(j.start).Seconds(); el > 0.5 {
			speed = fmt.Sprintf("  ·  %s/sn", fmtSize(int64(float64(done)/el)))
		}
		j.detail.SetText(fmt.Sprintf("%s / %s%s", fmtSize(done), fmtSize(total), speed))
	})
	return !j.canceled.Load()
}

// ask runs f on the UI thread and waits for its answer.
func (j *job) ask(f func(owner walk.Form) int) int {
	ch := make(chan int, 1)
	j.dlg.Synchronize(func() { ch <- f(j.dlg) })
	return <-ch
}

// runJob shows a progress window while work runs on another goroutine.
func runJob(owner walk.Form, title string, work func(j *job) error) error {
	j := &job{start: time.Now()}
	var cancelBtn *walk.PushButton
	finished := false
	var result error
	err := Dialog{
		AssignTo: &j.dlg,
		Title:    title,
		Icon:     appIcon,
		MinSize:  Size{Width: 460},
		Layout:   VBox{},
		Children: []Widget{
			Label{AssignTo: &j.label, Text: "Hazırlanıyor…", EllipsisMode: EllipsisPath},
			ProgressBar{AssignTo: &j.bar, MinValue: 0, MaxValue: 1000},
			Label{AssignTo: &j.detail, Text: " "},
			Composite{
				Layout: HBox{MarginsZero: true},
				Children: []Widget{
					HSpacer{},
					PushButton{AssignTo: &cancelBtn, Text: "İptal", OnClicked: func() {
						j.canceled.Store(true)
						cancelBtn.SetEnabled(false)
						cancelBtn.SetText("Durduruluyor…")
					}},
				},
			},
		},
	}.Create(owner)
	if err != nil {
		return err
	}
	j.dlg.Closing().Attach(func(canceled *bool, reason walk.CloseReason) {
		if !finished {
			*canceled = true
			j.canceled.Store(true)
		}
	})
	j.dlg.Starting().Attach(func() {
		go func() {
			r := work(j)
			j.dlg.Synchronize(func() {
				result = r
				finished = true
				j.dlg.Accept()
			})
		}()
	})
	j.dlg.Run()
	return result
}
