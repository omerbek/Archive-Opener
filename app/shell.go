//go:build windows

package main

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"unsafe"

	"github.com/lxn/walk"
	"github.com/lxn/win"
	"golang.org/x/sys/windows/registry"
)

const (
	fileAttrDirectory = 0x10
	fileAttrNormal    = 0x80
	shgfiUseFileAttrs = 0x10
)

var (
	iconMu    sync.Mutex
	iconCache = map[string]*walk.Icon{}
)

// shellIcon returns the small Explorer icon for a file name or a folder,
// without the file having to exist.
func shellIcon(name string, dir bool) *walk.Icon {
	key := "<dir>"
	attr := uint32(fileAttrDirectory)
	if !dir {
		ext := strings.ToLower(filepath.Ext(name))
		key = ext
		attr = fileAttrNormal
		if ext == ".exe" || ext == ".ico" || ext == ".lnk" {
			key = ext // per-file icons can't be known; use the generic one
		}
	}
	iconMu.Lock()
	defer iconMu.Unlock()
	if ic, ok := iconCache[key]; ok {
		return ic
	}
	probe := "dosya" + key
	if dir {
		probe = "klasor"
	}
	var shfi win.SHFILEINFO
	p, _ := syscall.UTF16PtrFromString(probe)
	var ic *walk.Icon
	if win.SHGetFileInfo(p, attr, &shfi, uint32(unsafe.Sizeof(shfi)),
		win.SHGFI_ICON|win.SHGFI_SMALLICON|shgfiUseFileAttrs) != 0 && shfi.HIcon != 0 {
		// The Icon takes ownership of the handle; it lives for the whole run.
		ic, _ = walk.NewIconFromHICON(shfi.HIcon)
	}
	iconCache[key] = ic
	return ic
}

// shellOpen opens a file or folder with its default program.
func shellOpen(path string) bool {
	verb, _ := syscall.UTF16PtrFromString("open")
	file, _ := syscall.UTF16PtrFromString(path)
	return win.ShellExecute(0, verb, file, nil, nil, win.SW_SHOWNORMAL)
}

// showInExplorer opens Explorer with the given folder.
func showInExplorer(dir string) {
	shellOpen(dir)
}

var (
	shell32            = syscall.NewLazyDLL("shell32.dll")
	procSHChangeNotify = shell32.NewProc("SHChangeNotify")
)

func notifyAssocChanged() {
	const shcneAssocChanged = 0x08000000
	procSHChangeNotify.Call(shcneAssocChanged, 0, 0, 0)
}

const progID = "ArsivAcici.Arsiv"

var assocExts = []string{".rar", ".zip", ".7z", ".tar", ".gz", ".tgz", ".bz2", ".tbz2", ".xz", ".txz", ".zst", ".r00", ".001"}

// registerAssociations adds the program to "Open with" for archive types and
// adds "Extract here" commands to their right-click menu, for this user only.
func registerAssociations() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	q := `"` + exe + `"`
	set := func(path, name, value string) error {
		k, _, err := registry.CreateKey(registry.CURRENT_USER, `Software\Classes\`+path, registry.SET_VALUE)
		if err != nil {
			return err
		}
		defer k.Close()
		return k.SetStringValue(name, value)
	}
	steps := [][3]string{
		{progID, "", "Arşiv"},
		{progID + `\DefaultIcon`, "", exe + ",0"},
		{progID + `\shell\open`, "FriendlyAppName", "Arşiv Açıcı"},
		{progID + `\shell\open\command`, "", q + ` "%1"`},
		{`Applications\` + filepath.Base(exe) + `\shell\open`, "FriendlyAppName", "Arşiv Açıcı"},
		{`Applications\` + filepath.Base(exe) + `\shell\open\command`, "", q + ` "%1"`},
	}
	for _, ext := range assocExts {
		steps = append(steps,
			[3]string{ext + `\OpenWithProgids`, progID, ""},
			[3]string{`SystemFileAssociations\` + ext + `\shell\ArsivAcici.Here`, "", "Buraya çıkar (Arşiv Açıcı)"},
			[3]string{`SystemFileAssociations\` + ext + `\shell\ArsivAcici.Here`, "Icon", exe + ",0"},
			[3]string{`SystemFileAssociations\` + ext + `\shell\ArsivAcici.Here\command`, "", q + ` -xh "%1"`},
			[3]string{`SystemFileAssociations\` + ext + `\shell\ArsivAcici.Folder`, "", "Klasöre çıkar (Arşiv Açıcı)"},
			[3]string{`SystemFileAssociations\` + ext + `\shell\ArsivAcici.Folder`, "Icon", exe + ",0"},
			[3]string{`SystemFileAssociations\` + ext + `\shell\ArsivAcici.Folder\command`, "", q + ` -x "%1"`},
		)
	}
	for _, s := range steps {
		if err := set(s[0], s[1], s[2]); err != nil {
			return err
		}
	}
	notifyAssocChanged()
	return nil
}

// unregisterAssociations removes everything registerAssociations added.
func unregisterAssociations() {
	exe, _ := os.Executable()
	delTree(`Software\Classes\` + progID)
	delTree(`Software\Classes\Applications\` + filepath.Base(exe))
	for _, ext := range assocExts {
		delTree(`Software\Classes\SystemFileAssociations\` + ext + `\shell\ArsivAcici.Here`)
		delTree(`Software\Classes\SystemFileAssociations\` + ext + `\shell\ArsivAcici.Folder`)
		if k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Classes\`+ext+`\OpenWithProgids`, registry.SET_VALUE); err == nil {
			k.DeleteValue(progID)
			k.Close()
		}
	}
	notifyAssocChanged()
}

func delTree(path string) {
	k, err := registry.OpenKey(registry.CURRENT_USER, path, registry.ENUMERATE_SUB_KEYS)
	if err == nil {
		names, _ := k.ReadSubKeyNames(-1)
		k.Close()
		for _, n := range names {
			delTree(path + `\` + n)
		}
	}
	registry.DeleteKey(registry.CURRENT_USER, path)
}

func associationsRegistered() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Classes\`+progID, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	k.Close()
	return true
}
