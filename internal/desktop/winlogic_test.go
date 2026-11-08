package desktop

import "testing"

func TestIsAppWindow(t *testing.T) {
	app := winInfo{Visible: true, Title: "Mozilla Firefox", Class: "MozillaWindowClass"}
	for name, c := range map[string]struct {
		w    winInfo
		want bool
	}{
		"plain app":       {app, true},
		"hidden":          {winInfo{Title: "x"}, false},
		"cloaked":         {winInfo{Visible: true, Cloaked: true, Title: "x"}, false},
		"untitled":        {winInfo{Visible: true}, false},
		"tool window":     {winInfo{Visible: true, Title: "x", ExStyle: wsExToolWindow}, false},
		"owned dialog":    {winInfo{Visible: true, Title: "Save as", Owned: true}, false},
		"owned appwindow": {winInfo{Visible: true, Title: "x", Owned: true, ExStyle: wsExAppWindow}, true},
		"no activate":     {winInfo{Visible: true, Title: "x", ExStyle: wsExNoActivate}, false},
		"desktop":         {winInfo{Visible: true, Title: "Program Manager", Class: "Progman"}, false},
	} {
		if got := isAppWindow(c.w); got != c.want {
			t.Errorf("%s: got %v", name, got)
		}
	}
}

func TestWinFocus(t *testing.T) {
	f := winFocus(`C:\Program Files\Mozilla Firefox\firefox.exe`, "Firefox", "Some page — Mozilla Firefox", "")
	if f != (Focus{ID: "firefox", Name: "Firefox", Class: "firefox"}) {
		t.Errorf("%+v", f)
	}
	f = winFocus(`D:\SteamLibrary\steamapps\common\Hades\x64\Hades.exe`, "", "Hades", "1145360")
	if f != (Focus{ID: "hades", Name: "Hades", Class: "steam_app_1145360"}) {
		t.Errorf("%+v", f)
	}
	if f := winFocus(`C:\x\Code.EXE`, "", "", ""); f.ID != "code" || f.Name != "code" {
		t.Errorf("%+v", f)
	}
	if !ignoredFocus("Shell_TrayWnd", "explorer") || !ignoredFocus("Windows.UI.Core.CoreWindow", "searchhost") || ignoredFocus("CabinetWClass", "explorer") {
		t.Error("ignoredFocus")
	}
}

func TestClipboardPrivate(t *testing.T) {
	if clipboardPrivate(map[string]bool{"HTML Format": true}, nil) {
		t.Error("ordinary text is not private")
	}
	if !clipboardPrivate(map[string]bool{"ExcludeClipboardContentFromMonitorProcessing": true}, nil) {
		t.Error("exclude format")
	}
	if !clipboardPrivate(map[string]bool{"CanIncludeInClipboardHistory": true}, []byte{0, 0, 0, 0}) {
		t.Error("CanIncludeInClipboardHistory = 0")
	}
	if clipboardPrivate(map[string]bool{"CanIncludeInClipboardHistory": true}, []byte{1, 0, 0, 0}) {
		t.Error("CanIncludeInClipboardHistory = 1")
	}
}

func TestPlayerName(t *testing.T) {
	for in, want := range map[string]string{
		"Spotify.exe": "Spotify",
		"Microsoft.ZuneMusic_8wekyb3d8bbwe!Microsoft.ZuneMusic": "ZuneMusic",
		"308046B0AF4A39CB": "308046B0AF4A39CB",
		"chrome":           "chrome",
	} {
		if got := playerName(in); got != want {
			t.Errorf("%s: got %q", in, got)
		}
	}
}
