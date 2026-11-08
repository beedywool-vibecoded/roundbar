package desktop

import (
	"errors"
	"testing"
)

func TestPick(t *testing.T) {
	for _, c := range []struct {
		env  map[string]string
		want string
	}{
		{map[string]string{"XDG_CURRENT_DESKTOP": "GNOME", "WAYLAND_DISPLAY": "wayland-0"}, BackendGnome},
		{map[string]string{"XDG_CURRENT_DESKTOP": "ubuntu:GNOME", "DISPLAY": ":0"}, BackendGnome},
		{map[string]string{"XDG_CURRENT_DESKTOP": "Budgie:GNOME", "DISPLAY": ":0"}, BackendX11},
		{map[string]string{"XDG_CURRENT_DESKTOP": "GNOME-Flashback:GNOME", "DISPLAY": ":0"}, BackendX11},
		{map[string]string{"XDG_CURRENT_DESKTOP": "KDE", "WAYLAND_DISPLAY": "wayland-0"}, BackendKDE},
		{map[string]string{"XDG_CURRENT_DESKTOP": "Hyprland", "WAYLAND_DISPLAY": "wayland-1"}, BackendWayland},
		{map[string]string{"XDG_CURRENT_DESKTOP": "sway", "WAYLAND_DISPLAY": "wayland-1", "DISPLAY": ":0"}, BackendWayland},
		{map[string]string{"XDG_CURRENT_DESKTOP": "XFCE", "DISPLAY": ":0"}, BackendX11},
		{map[string]string{"XDG_CURRENT_DESKTOP": "X-Cinnamon", "DISPLAY": ":0"}, BackendX11},
		{map[string]string{"XDG_CURRENT_DESKTOP": "i3", "DISPLAY": ":0"}, BackendX11},
		{map[string]string{"XDG_CURRENT_DESKTOP": "Hyprland", "DISPLAY": ":1"}, BackendWayland},
		{map[string]string{"XDG_CURRENT_DESKTOP": "Weird"}, BackendNone},
		{map[string]string{}, BackendGnome},
		{map[string]string{"XDG_CURRENT_DESKTOP": "GNOME", "ROUNDBAR_DESKTOP": "X11"}, BackendX11},
	} {
		if got := pick(func(k string) string { return c.env[k] }); got != c.want {
			t.Errorf("%v: got %s, want %s", c.env, got, c.want)
		}
	}
}

type fakeShell struct {
	none
	caps Caps
	name string
}

func (f *fakeShell) Caps() Caps { return f.caps }
func (f *fakeShell) Clipboard() ([]string, error) {
	return []string{f.name}, nil
}

func TestMix(t *testing.T) {
	win := &fakeShell{caps: Caps{Focus: true, Windows: true}, name: "win"}
	clip := &fakeShell{caps: Caps{Clipboard: true}, name: "clip"}
	m := mix{win: win, notif: none{}, clip: clip}
	if c := m.Caps(); c != (Caps{Focus: true, Windows: true, Clipboard: true}) {
		t.Fatalf("caps %+v", c)
	}
	if l, _ := m.Clipboard(); l[0] != "clip" {
		t.Fatal(l)
	}
	if _, err := m.Notifications(); !errors.Is(err, ErrUnsupported) {
		t.Fatal(err)
	}
	m.Close()
}

func TestFocusFor(t *testing.T) {
	testApps(t, map[string]string{
		"org.mozilla.firefox": "[Desktop Entry]\nName=Firefox\nIcon=firefox\nStartupWMClass=firefox\n",
		"org.gnome.Nautilus":  "[Desktop Entry]\nName=Files\nIcon=org.gnome.Nautilus\n",
		"code":                "[Desktop Entry]\nName=Visual Studio Code\nIcon=vscode\nStartupWMClass=Code\n[Desktop Action new]\nName=New Window\n",
		"hidden":              "[Desktop Entry]\nName=Hidden\nHidden=true\n",
	})
	for _, c := range []struct {
		desktop, class, instance, title string
		want                            Focus
	}{
		{"", "firefox", "Navigator", "Some page — Mozilla Firefox", Focus{"org.mozilla.firefox", "Firefox", "firefox"}},
		{"", "org.gnome.Nautilus", "", "Home", Focus{"org.gnome.nautilus", "Files", "org.gnome.nautilus"}},
		{"", "Code", "code", "main.go", Focus{"code", "Visual Studio Code", "code"}},
		{"", "steam_app_1091500", "steam_app_1091500", "Cyberpunk 2077", Focus{"steam_app_1091500", "Cyberpunk 2077", "steam_app_1091500"}},
		{"", "nautilus", "", "Home", Focus{"org.gnome.nautilus", "Files", "nautilus"}}, // last part of a reverse-DNS id
		{"org.gnome.Nautilus.desktop", "whatever", "", "", Focus{"org.gnome.nautilus", "Files", "whatever"}},
		{"Hidden", "hidden", "", "x", Focus{"hidden", "x", "hidden"}},
		{"", "Unknown", "", "", Focus{"unknown", "Unknown", "unknown"}},
	} {
		if got := focusFor(c.desktop, c.class, c.instance, c.title); got != c.want {
			t.Errorf("%s/%s: got %+v, want %+v", c.desktop, c.class, got, c.want)
		}
	}
	w := windowFor("1", "", "firefox", "", "Page")
	if w != (Window{ID: "1", App: "org.mozilla.firefox", Title: "Page", Icon: "firefox"}) {
		t.Errorf("window %+v", w)
	}
}

func TestPushHistory(t *testing.T) {
	var l []string
	for _, s := range []string{"a", "b", "  ", "a", "c"} {
		l, _ = pushHistory(l, s)
	}
	if len(l) != 3 || l[0] != "c" || l[1] != "a" || l[2] != "b" {
		t.Fatal(l)
	}
	if _, changed := pushHistory(l, "c"); changed {
		t.Fatal("same text again should not count as a change")
	}
	for i := 0; i < 20; i++ {
		l, _ = pushHistory(l, string(rune('d'+i)))
	}
	if len(l) != maxClips {
		t.Fatal(len(l))
	}
	long := make([]rune, 3000)
	for i := range long {
		long[i] = 'é'
	}
	l, _ = pushHistory(nil, string(long))
	if len([]rune(l[0])) != maxClipLen {
		t.Fatal(len([]rune(l[0])))
	}
}
