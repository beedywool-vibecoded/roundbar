package desktop

import (
	"encoding/binary"
	"path/filepath"
	"strings"
)

// Decisions of the Windows backend that are plain logic (tested on every OS).

// winInfo is what the Windows backend knows about a top-level window.
type winInfo struct {
	Visible, Cloaked bool
	Owned            bool // has an owner window (dialogs, palettes)
	ExStyle          uint32
	Title, Class     string
}

const (
	wsExToolWindow = 0x00000080
	wsExAppWindow  = 0x00040000
	wsExNoActivate = 0x08000000
)

// shellClasses are the desktop and taskbar, not apps.
var shellClasses = map[string]bool{"Progman": true, "WorkerW": true, "Shell_TrayWnd": true, "Shell_SecondaryTrayWnd": true}

// isAppWindow decides what Alt+Tab would list: visible, not cloaked (other
// virtual desktops, suspended UWP frames), titled, and either forced in with
// WS_EX_APPWINDOW or unowned and neither a tool window nor non-activating.
func isAppWindow(w winInfo) bool {
	if !w.Visible || w.Cloaked || strings.TrimSpace(w.Title) == "" || shellClasses[w.Class] {
		return false
	}
	if w.ExStyle&wsExAppWindow != 0 {
		return true
	}
	return !w.Owned && w.ExStyle&(wsExToolWindow|wsExNoActivate) == 0
}

// ignoredFocus: the desktop, the taskbar, Alt+Tab and the Start menu take the
// foreground without being an app; the profile stays where it was.
func ignoredFocus(class, exeID string) bool {
	switch class {
	case "Progman", "WorkerW", "Shell_TrayWnd", "Shell_SecondaryTrayWnd", "MultitaskingViewFrame",
		"XamlExplorerHostIslandWindow", "ForegroundStaging", "TaskSwitcherWnd", "NotifyIconOverflowWindow":
		return true
	}
	switch exeID {
	case "searchhost", "searchapp", "startmenuexperiencehost", "shellexperiencehost", "lockapp":
		return true
	}
	return false
}

// winFocus builds the Focus of a window: ID and Class are the lowercase
// executable name without .exe ("firefox", "code"), Class is
// "steam_app_<id>" for Steam games so profiles written for Linux match; Name
// is the executable's FileDescription, else the window title.
func winFocus(exe, description, title, steamApp string) Focus {
	id := strings.TrimSuffix(strings.ToLower(filepath.Base(strings.ReplaceAll(exe, `\`, "/"))), ".exe")
	f := Focus{ID: id, Class: id, Name: strings.TrimSpace(description)}
	if steamApp != "" {
		f.Class = "steam_app_" + steamApp
	}
	if f.Name == "" {
		f.Name = strings.TrimSpace(title)
	}
	if f.Name == "" {
		f.Name = id
	}
	return f
}

// clipboardPrivate: password managers mark what they copy so clipboard
// tools leave it alone ("ExcludeClipboardContentFromMonitorProcessing",
// "Clipboard Viewer Ignore", or "CanIncludeInClipboardHistory" set to 0).
// present holds the registered format names on the clipboard, canInclude the
// data of CanIncludeInClipboardHistory (nil if absent).
func clipboardPrivate(present map[string]bool, canInclude []byte) bool {
	if present["ExcludeClipboardContentFromMonitorProcessing"] || present["Clipboard Viewer Ignore"] {
		return true
	}
	return len(canInclude) >= 4 && binary.LittleEndian.Uint32(canInclude) == 0
}

// playerName makes a media session's app id readable: "Spotify.exe" ->
// "Spotify", "Microsoft.ZuneMusic_8wekyb3d8bbwe!Microsoft.ZuneMusic" -> "ZuneMusic".
func playerName(aumid string) string {
	s := aumid
	if i := strings.LastIndexByte(s, '!'); i >= 0 {
		s = s[i+1:]
	}
	s = strings.TrimSuffix(strings.TrimSuffix(s, ".exe"), ".EXE")
	if i := strings.LastIndexByte(s, '.'); i >= 0 && i < len(s)-1 {
		s = s[i+1:]
	}
	return s
}
