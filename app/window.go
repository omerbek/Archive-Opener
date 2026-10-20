//go:build windows

package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"

	"arsivacici/arc"
)

type app struct {
	mw        *walk.MainWindow
	tv        *walk.TableView
	model     *listModel
	pathEdit  *walk.LineEdit
	listPane  *walk.Composite
	emptyPane *walk.Composite
	status    *walk.StatusBarItem
	selStatus *walk.StatusBarItem

	actExtract, actExtractFolder, actTest, actInfo, actUp, actOpenItem *walk.Action
	actAssoc                                                          *walk.Action

	path     string
	password string
	a        arc.Archive
	root     *node
	cwd      *node
	busy     bool
	tempDir  string
}

var archiveFilter = "Arşivler (*.rar;*.zip;*.7z;*.tar;*.gz;*.tgz;*.bz2;*.tbz2;*.xz;*.txz;*.zst;*.exe;*.001)|*.rar;*.zip;*.7z;*.tar;*.gz;*.tgz;*.bz2;*.tbz2;*.xz;*.txz;*.zst;*.exe;*.001|Tüm dosyalar (*.*)|*.*"

func (a *app) run(start string) {
	a.model = &listModel{}
	ctrl := func(k walk.Key) Shortcut { return Shortcut{Modifiers: walk.ModControl, Key: k} }

	err := MainWindow{
		AssignTo:   &a.mw,
		Title:      appName,
		Icon:       appIcon,
		Size:       Size{Width: 920, Height: 600},
		MinSize:    Size{Width: 520, Height: 320},
		Persistent: true,
		Layout:     VBox{MarginsZero: true, SpacingZero: true},
		OnDropFiles: func(files []string) {
			if len(files) > 0 {
				a.open(files[0])
			}
		},
		MenuItems: []MenuItem{
			Menu{
				Text: "&Dosya",
				Items: []MenuItem{
					Action{Text: "&Arşiv aç…", Shortcut: ctrl(walk.KeyO), OnTriggered: a.browseOpen},
					Action{AssignTo: &a.actInfo, Text: "Arşiv &bilgisi", Shortcut: ctrl(walk.KeyI), Enabled: false, OnTriggered: a.showInfo},
					Separator{},
					Action{Text: "Çı&kış", OnTriggered: func() { a.mw.Close() }},
				},
			},
			Menu{
				Text: "&Komutlar",
				Items: []MenuItem{
					Action{AssignTo: &a.actOpenItem, Text: "Seçileni a&ç", Enabled: false, OnTriggered: a.activate},
					Action{AssignTo: &a.actExtract, Text: "&Çıkar…", Shortcut: Shortcut{Modifiers: walk.ModAlt, Key: walk.KeyE}, Enabled: false, OnTriggered: func() { a.extract(true) }},
					Action{AssignTo: &a.actExtractFolder, Text: "&Klasöre çıkar", Enabled: false, OnTriggered: func() { a.extract(false) }},
					Action{AssignTo: &a.actTest, Text: "Arşivi &test et", Shortcut: Shortcut{Modifiers: walk.ModAlt, Key: walk.KeyT}, Enabled: false, OnTriggered: a.test},
					Separator{},
					Action{AssignTo: &a.actUp, Text: "Ü&st klasör", Enabled: false, OnTriggered: a.up},
				},
			},
			Menu{
				Text: "&Seçenekler",
				Items: []MenuItem{
					Action{AssignTo: &a.actAssoc, Text: "Arşivleri Windows'ta bu programla aç…", OnTriggered: a.toggleAssoc},
				},
			},
			Menu{
				Text: "&Yardım",
				Items: []MenuItem{
					Action{Text: "&Hakkında", OnTriggered: a.about},
				},
			},
		},
		ToolBar: ToolBar{
			ButtonStyle: ToolBarButtonTextOnly,
			Items: []MenuItem{
				Action{Text: "  Aç  ", OnTriggered: a.browseOpen},
				Separator{},
				Action{Text: "  Çıkar…  ", OnTriggered: func() { a.extract(true) }},
				Action{Text: "  Klasöre çıkar  ", OnTriggered: func() { a.extract(false) }},
				Action{Text: "  Test et  ", OnTriggered: a.test},
				Action{Text: "  Bilgi  ", OnTriggered: a.showInfo},
				Separator{},
				Action{Text: "  ↑ Yukarı  ", OnTriggered: a.up},
			},
		},
		Children: []Widget{
			Composite{
				AssignTo: &a.emptyPane,
				Layout:   VBox{Margins: Margins{Left: 40, Top: 40, Right: 40, Bottom: 40}},
				Children: []Widget{
					VSpacer{},
					Label{Text: "Bir arşivi bu pencereye sürükleyin", Font: Font{PointSize: 16}, Alignment: AlignHCenterVCenter},
					Label{Text: "ya da araç çubuğundaki “Aç” düğmesine tıklayın.", Font: Font{PointSize: 11}, Alignment: AlignHCenterVCenter},
					VSpacer{Size: 12},
					Label{Text: "RAR · RAR5 · ZIP · 7Z · TAR · GZ · BZ2 · XZ · ZST · çok parçalı RAR · kendi kendini açan (SFX) .exe",
						Alignment: AlignHCenterVCenter, TextColor: walk.RGB(110, 110, 110)},
					PushButton{Text: "Arşiv aç…", OnClicked: a.browseOpen, MaxSize: Size{Width: 160}},
					VSpacer{},
				},
			},
			Composite{
				AssignTo: &a.listPane,
				Visible:  false,
				Layout:   VBox{Margins: Margins{Left: 6, Top: 6, Right: 6, Bottom: 0}, Spacing: 4},
				Children: []Widget{
					Composite{
						Layout: HBox{MarginsZero: true},
						Children: []Widget{
							PushButton{Text: "↑", MaxSize: Size{Width: 32}, ToolTipText: "Üst klasör (Geri)", OnClicked: a.up},
							LineEdit{AssignTo: &a.pathEdit, ReadOnly: true},
						},
					},
					TableView{
						AssignTo:            &a.tv,
						Model:               a.model,
						MultiSelection:      true,
						AlternatingRowBG:    true,
						ColumnsOrderable:    true,
						LastColumnStretched: false,
						Columns: []TableViewColumn{
							{Title: "Ad", Width: 330},
							{Title: "Boyut", Width: 90, Alignment: AlignFar},
							{Title: "Sıkıştırılmış", Width: 100, Alignment: AlignFar},
							{Title: "Tür", Width: 140},
							{Title: "Değiştirilme", Width: 130},
						},
						OnItemActivated:          a.activate,
						OnSelectedIndexesChanged: a.updateSelStatus,
						OnKeyDown: func(key walk.Key) {
							switch key {
							case walk.KeyBack:
								a.up()
							case walk.KeyReturn:
								a.activate()
							}
						},
						ContextMenuItems: []MenuItem{
							Action{Text: "Aç", OnTriggered: a.activate},
							Separator{},
							Action{Text: "Seçilenleri çıkar…", OnTriggered: func() { a.extract(true) }},
							Action{Text: "Seçilenleri klasöre çıkar", OnTriggered: func() { a.extract(false) }},
						},
					},
				},
			},
		},
		StatusBarItems: []StatusBarItem{
			{AssignTo: &a.status, Text: "Hazır", Width: 420},
			{AssignTo: &a.selStatus, Text: "", Width: 260},
		},
	}.Create()
	if err != nil {
		walk.MsgBox(nil, appName, "Pencere oluşturulamadı: "+err.Error(), walk.MsgBoxIconError)
		return
	}
	a.updateAssocText()
	if start != "" {
		a.mw.Starting().Attach(func() { a.open(start) })
	}
	a.mw.Run()
	if a.a != nil {
		a.a.Close()
	}
	if a.tempDir != "" {
		os.RemoveAll(a.tempDir)
	}
}

func (a *app) setStatus(s string) { a.status.SetText(s) }

func (a *app) browseOpen() {
	fd := &walk.FileDialog{Title: "Arşiv aç", Filter: archiveFilter}
	if a.path != "" {
		fd.InitialDirPath = filepath.Dir(a.path)
	}
	if ok, _ := fd.ShowOpen(a.mw); ok {
		a.open(fd.FilePath)
	}
}

// open reads the archive list on a background goroutine, asking for a
// password when the file list itself is encrypted.
func (a *app) open(path string) {
	if a.busy {
		return
	}
	a.busy = true
	a.setStatus("Açılıyor: " + filepath.Base(path) + " …")
	a.mw.SetEnabled(false)
	var tryOpen func(pw string, wrong bool)
	tryOpen = func(pw string, wrong bool) {
		go func() {
			ar, err := arc.Open(path, pw)
			a.mw.Synchronize(func() {
				if errors.Is(err, arc.ErrNeedPassword) || errors.Is(err, arc.ErrBadPassword) {
					p, ok := askPassword(a.mw, filepath.Base(path), wrong || errors.Is(err, arc.ErrBadPassword))
					if ok {
						tryOpen(p, true)
						return
					}
					a.finishOpen(path, nil, "", nil)
					return
				}
				a.finishOpen(path, ar, pw, err)
			})
		}()
	}
	tryOpen("", false)
}

func (a *app) finishOpen(path string, ar arc.Archive, pw string, err error) {
	a.busy = false
	a.mw.SetEnabled(true)
	if ar == nil {
		if err != nil {
			walk.MsgBox(a.mw, appName, fmt.Sprintf("\"%s\" açılamadı.\n\n%s", filepath.Base(path), friendly(err)), walk.MsgBoxIconError)
		}
		if a.a == nil {
			a.setStatus("Hazır")
		} else {
			a.setStatus(a.summary())
		}
		return
	}
	if a.a != nil {
		a.a.Close()
	}
	a.a, a.path, a.password = ar, path, pw
	a.root = buildTree(ar.Entries())
	a.cwd = a.root
	a.emptyPane.SetVisible(false)
	a.listPane.SetVisible(true)
	for _, act := range []*walk.Action{a.actExtract, a.actExtractFolder, a.actTest, a.actInfo, a.actOpenItem} {
		act.SetEnabled(true)
	}
	a.mw.SetTitle(filepath.Base(path) + " — " + appName)
	a.showFolder(a.root)
	a.setStatus(a.summary())
	a.tv.SetFocus()
}

func (a *app) summary() string {
	if a.root == nil {
		return "Hazır"
	}
	return fmt.Sprintf("%s  ·  %d dosya, %d klasör, toplam %s", a.a.Format(), a.root.files, a.root.countDirs(), fmtSize(a.root.size))
}

func (a *app) showFolder(n *node) {
	a.cwd = n
	a.model.setFolder(n)
	shown := filepath.Base(a.path)
	if n.path != "" {
		shown += `\` + strings.ReplaceAll(strings.TrimSuffix(n.path, "/"), "/", `\`)
	}
	a.pathEdit.SetText(shown)
	a.actUp.SetEnabled(n.parent != nil)
	if len(a.model.rows) > 0 {
		a.tv.SetCurrentIndex(0)
	}
	a.updateSelStatus()
}

func (a *app) up() {
	if a.cwd == nil || a.cwd.parent == nil {
		return
	}
	from := a.cwd
	a.showFolder(a.cwd.parent)
	for i, r := range a.model.rows {
		if r.n == from {
			a.tv.SetCurrentIndex(i)
			break
		}
	}
}

func (a *app) selectedNodes() []*node {
	var out []*node
	for _, i := range a.tv.SelectedIndexes() {
		if i >= 0 && i < len(a.model.rows) && !a.model.rows[i].up {
			out = append(out, a.model.rows[i].n)
		}
	}
	return out
}

func (a *app) updateSelStatus() {
	sel := a.selectedNodes()
	if len(sel) == 0 {
		a.selStatus.SetText("")
		return
	}
	var size int64
	files := 0
	for _, n := range sel {
		size += n.size
		files += n.files
	}
	a.selStatus.SetText(fmt.Sprintf("Seçili: %d öğe, %d dosya, %s", len(sel), files, fmtSize(size)))
}

// activate opens a folder, goes up, or opens a file with its program.
func (a *app) activate() {
	if a.a == nil {
		return
	}
	i := a.tv.CurrentIndex()
	if i < 0 || i >= len(a.model.rows) {
		return
	}
	r := a.model.rows[i]
	if r.up {
		a.up()
		return
	}
	if r.n.dir {
		a.showFolder(r.n)
		return
	}
	a.openEntry(r.n)
}

// wantFor returns a selector for the given nodes (folders include
// everything beneath them).
func wantFor(nodes []*node) func(*arc.Entry) bool {
	files := map[*arc.Entry]bool{}
	var prefixes []string
	for _, n := range nodes {
		if n.dir {
			prefixes = append(prefixes, n.path)
		} else if n.entry != nil {
			files[n.entry] = true
		}
	}
	return func(e *arc.Entry) bool {
		if files[e] {
			return true
		}
		for _, p := range prefixes {
			if strings.HasPrefix(e.Path, p) {
				return true
			}
		}
		return false
	}
}

const (
	pwAuto  = iota // ask only if the chosen files are marked encrypted
	pwNeed         // the archive said a password is needed
	pwWrong        // the last password was wrong
)

// ensurePassword asks for a password when needed, then reopens the
// archive with it.
func (a *app) ensurePassword(want func(*arc.Entry) bool, mode int) bool {
	if mode == pwAuto && (a.password != "" || !anyEncrypted(a.a.Entries(), want)) {
		return true
	}
	pw, ok := askPassword(a.mw, filepath.Base(a.path), mode == pwWrong)
	if !ok {
		return false
	}
	ar, err := arc.Open(a.path, pw)
	if err != nil {
		walk.MsgBox(a.mw, appName, friendly(err), walk.MsgBoxIconError)
		return false
	}
	a.a.Close()
	a.a, a.password = ar, pw
	// Entries are new objects now; rebuild the tree but stay in the same folder.
	cwdPath := a.cwd.path
	a.root = buildTree(ar.Entries())
	if n := a.root.find(cwdPath); n != nil {
		a.showFolder(n)
	} else {
		a.showFolder(a.root)
	}
	return true
}

func (a *app) openEntry(n *node) {
	if a.tempDir == "" {
		d, err := os.MkdirTemp("", "ArsivAcici-")
		if err != nil {
			walk.MsgBox(a.mw, appName, "Geçici klasör oluşturulamadı: "+err.Error(), walk.MsgBoxIconError)
			return
		}
		a.tempDir = d
	}
	mode := pwAuto
	for {
		target := n.path
		want := func(e *arc.Entry) bool { return e.Path == target }
		if !a.ensurePassword(want, mode) {
			return
		}
		dest, err := os.MkdirTemp(a.tempDir, "f")
		if err != nil {
			return
		}
		prefix := ""
		if i := strings.LastIndex(target, "/"); i >= 0 {
			prefix = target[:i+1]
		}
		err = extractWithJob(a.mw, a.a, dest, arc.ExtractOptions{Want: want, StripPrefix: prefix}, 1)
		if errors.Is(err, arc.ErrBadPassword) {
			mode = pwWrong
			continue
		}
		if errors.Is(err, arc.ErrNeedPassword) {
			mode = pwNeed
			continue
		}
		if err != nil {
			if !errors.Is(err, arc.ErrCanceled) {
				walk.MsgBox(a.mw, appName, friendly(err), walk.MsgBoxIconError)
			}
			return
		}
		rel, _ := arc.SafeRel(n.name)
		file := filepath.Join(dest, rel)
		if isArchiveName(n.name) {
			a.open(file)
			return
		}
		if !shellOpen(file) {
			walk.MsgBox(a.mw, appName, "Dosya açılamadı. Bu dosya türüyle ilişkili bir program olmayabilir.", walk.MsgBoxIconWarning)
		}
		return
	}
}

func isArchiveName(name string) bool {
	l := strings.ToLower(name)
	for _, e := range assocExts {
		if strings.HasSuffix(l, e) {
			return true
		}
	}
	return false
}

// extract extracts the selection (or everything when nothing is selected).
// withDialog shows the destination/options window first.
func (a *app) extract(withDialog bool) {
	if a.a == nil || a.busy {
		return
	}
	sel := a.selectedNodes()
	var want func(*arc.Entry) bool
	prefix := ""
	what := fmt.Sprintf("Arşivin tamamı (%d dosya, %s)", a.root.files, fmtSize(a.root.size))
	if len(sel) > 0 && !(len(sel) == len(a.cwd.children) && a.cwd == a.root) {
		want = wantFor(sel)
		prefix = a.cwd.path
		files := 0
		var size int64
		for _, n := range sel {
			files += n.files
			size += n.size
		}
		what = fmt.Sprintf("Seçilen %d öğe (%d dosya, %s)", len(sel), files, fmtSize(size))
	}
	dest := filepath.Join(filepath.Dir(a.path), archiveBaseName(a.path))
	choice := extractChoice{dest: dest, openFolder: true}
	if withDialog {
		var ok bool
		choice, ok = askExtractOptions(a.mw, dest, what)
		if !ok {
			return
		}
	}
	mode := pwAuto
	for {
		if !a.ensurePassword(want, mode) {
			return
		}
		// ensurePassword may have reopened the archive; rebuild the selector.
		if want != nil {
			want = wantFor(a.reselect(sel))
		}
		err := extractWithJob(a.mw, a.a, choice.dest, arc.ExtractOptions{Want: want, StripPrefix: prefix}, choice.conflict)
		switch {
		case errors.Is(err, arc.ErrBadPassword):
			mode = pwWrong
			continue
		case errors.Is(err, arc.ErrNeedPassword):
			mode = pwNeed
			continue
		case errors.Is(err, arc.ErrCanceled):
			a.setStatus("Çıkarma iptal edildi.")
		case err != nil:
			walk.MsgBox(a.mw, appName, "Çıkarma tamamlanamadı.\n\n"+friendly(err), walk.MsgBoxIconError)
			a.setStatus("Çıkarma tamamlanamadı.")
		default:
			a.setStatus("Çıkarıldı: " + choice.dest)
			if choice.openFolder {
				showInExplorer(choice.dest)
			}
		}
		return
	}
}

// reselect maps nodes from before a reopen onto the current tree by path.
func (a *app) reselect(old []*node) []*node {
	var out []*node
	for _, n := range old {
		if n.dir {
			if m := a.root.find(n.path); m != nil {
				out = append(out, m)
			}
			continue
		}
		dir := a.root.find(n.parent.path)
		if dir != nil {
			if m := dir.children["\x00"+n.name]; m != nil {
				out = append(out, m)
			}
		}
	}
	return out
}

func (a *app) test() {
	if a.a == nil || a.busy {
		return
	}
	mode := pwAuto
	for {
		if !a.ensurePassword(nil, mode) {
			return
		}
		total := arc.TotalSize(a.a, nil)
		err := runJob(a.mw, "Test ediliyor — "+appName, func(j *job) error {
			return arc.Test(a.a, func(done int64, name string) bool { return j.update(done, total, name) })
		})
		switch {
		case errors.Is(err, arc.ErrBadPassword):
			mode = pwWrong
			continue
		case errors.Is(err, arc.ErrNeedPassword):
			mode = pwNeed
			continue
		case errors.Is(err, arc.ErrStop):
			a.setStatus("Test iptal edildi.")
		case err != nil:
			walk.MsgBox(a.mw, "Test sonucu", "Arşivde hata bulundu:\n\n"+friendly(err), walk.MsgBoxIconError)
		default:
			walk.MsgBox(a.mw, "Test sonucu", fmt.Sprintf("Hata bulunmadı.\n\n%d dosya, %s başarıyla okundu.", a.root.files, fmtSize(total)), walk.MsgBoxIconInformation)
		}
		return
	}
}

func (a *app) showInfo() {
	if a.a == nil {
		return
	}
	st, _ := os.Stat(a.path)
	var arcSize int64
	if st != nil {
		arcSize = st.Size()
	}
	ratio := ""
	if a.root.size > 0 && arcSize > 0 {
		ratio = fmt.Sprintf("%%%d", arcSize*100/a.root.size)
	}
	enc := "Hayır"
	if anyEncrypted(a.a.Entries(), nil) || a.password != "" {
		enc = "Evet"
	}
	msg := fmt.Sprintf("Dosya:\t%s\nKlasör:\t%s\nBiçim:\t%s\n\nDosya sayısı:\t%d\nKlasör sayısı:\t%d\nAçılmış boyut:\t%s\nArşiv boyutu:\t%s\nSıkıştırma oranı:\t%s\nŞifreli:\t%s",
		filepath.Base(a.path), filepath.Dir(a.path), a.a.Format(),
		a.root.files, a.root.countDirs(), fmtSize(a.root.size), fmtSize(arcSize), ratio, enc)
	walk.MsgBox(a.mw, "Arşiv bilgisi", msg, walk.MsgBoxIconInformation)
}

func (a *app) updateAssocText() {
	if associationsRegistered() {
		a.actAssoc.SetText("Windows ilişkilendirmesini kaldır")
	} else {
		a.actAssoc.SetText("Arşivleri Windows'ta bu programla aç…")
	}
}

func (a *app) toggleAssoc() {
	if associationsRegistered() {
		unregisterAssociations()
		a.updateAssocText()
		walk.MsgBox(a.mw, appName, "Windows ilişkilendirmesi kaldırıldı.", walk.MsgBoxIconInformation)
		return
	}
	r := walk.MsgBox(a.mw, appName,
		"Bu işlem yalnızca sizin kullanıcı hesabınız için şunları ekler:\n\n"+
			"• Arşivlere sağ tıklayınca “Birlikte aç” listesinde Arşiv Açıcı\n"+
			"• Sağ tık menüsünde “Buraya çıkar” ve “Klasöre çıkar”\n\n"+
			"Programın şu anki konumu kaydedilir; exe dosyasını taşırsanız bu işlemi tekrarlayın. Devam edilsin mi?",
		walk.MsgBoxYesNo|walk.MsgBoxIconQuestion)
	if r != walk.DlgCmdYes {
		return
	}
	if err := registerAssociations(); err != nil {
		walk.MsgBox(a.mw, appName, "Kayıt yapılamadı: "+err.Error(), walk.MsgBoxIconError)
		return
	}
	a.updateAssocText()
	walk.MsgBox(a.mw, appName,
		"Tamam. Bir .rar dosyasına sağ tıklayıp “Birlikte aç → Arşiv Açıcı”yı seçin ve “Her zaman bu uygulamayı kullan”ı işaretlerseniz arşivler çift tıklayınca bu programla açılır.\n\n"+
			"Windows 11'de “Buraya çıkar” komutları “Diğer seçenekleri göster” altında görünür.",
		walk.MsgBoxIconInformation)
}

func (a *app) about() {
	walk.MsgBox(a.mw, "Hakkında",
		appName+" "+appVersion+"\n\n"+
			"RAR, RAR5, ZIP, 7z, TAR, GZ, BZ2, XZ ve ZST arşivlerini açar.\n"+
			"Çok parçalı RAR, şifreli RAR/7z/ZIP (ZipCrypto ve AES) ve kendi kendini açan .exe arşivleri desteklenir.\n\n"+
			"Bu program arşiv açar; arşiv oluşturmaz.",
		walk.MsgBoxIconInformation)
}
