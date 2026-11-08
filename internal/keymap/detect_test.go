package keymap

import (
	"errors"
	"os"
	"strings"
	"testing"
)

type fakeEnv struct {
	vars  map[string]string
	files map[string]string
	cmds  map[string]string // "name arg arg" -> output
	kde   int
}

func (f fakeEnv) env() Env {
	return Env{
		Getenv: func(k string) string { return f.vars[k] },
		ReadFile: func(p string) ([]byte, error) {
			if s, ok := f.files[p]; ok {
				return []byte(s), nil
			}
			return nil, os.ErrNotExist
		},
		Run: func(name string, args ...string) ([]byte, error) {
			if s, ok := f.cmds[strings.Join(append([]string{name}, args...), " ")]; ok {
				return []byte(s), nil
			}
			return nil, errors.New("not found")
		},
		KDEIndex: func() (int, bool) { return f.kde, f.kde >= 0 },
	}
}

func TestDetect(t *testing.T) {
	gs := "gsettings get org.gnome.desktop.input-sources "
	cases := []struct {
		name string
		env  fakeEnv
		want Detected
	}{
		{"gnome mru", fakeEnv{vars: map[string]string{"XDG_CURRENT_DESKTOP": "GNOME"}, cmds: map[string]string{
			gs + "mru-sources": "[('xkb', 'de'), ('xkb', 'fr')]\n", gs + "sources": "[('xkb', 'fr'), ('xkb', 'de')]\n"}},
			Detected{"de", "gnome"}},
		{"gnome empty mru, variant", fakeEnv{vars: map[string]string{"XDG_CURRENT_DESKTOP": "ubuntu:GNOME"}, cmds: map[string]string{
			gs + "mru-sources": "@a(ss) []\n", gs + "sources": "[('xkb', 'ch+fr')]\n"}},
			Detected{"ch-fr", "gnome"}},
		{"gnome ibus first", fakeEnv{vars: map[string]string{"XDG_CURRENT_DESKTOP": "GNOME"}, cmds: map[string]string{
			gs + "mru-sources": "[('ibus', 'mozc-jp'), ('xkb', 'gb')]\n"}},
			Detected{"gb", "gnome"}},
		{"kde active index", fakeEnv{vars: map[string]string{"XDG_CURRENT_DESKTOP": "KDE", "HOME": "/home/u"}, kde: 1, files: map[string]string{
			"/home/u/.config/kxkbrc": "[Layout]\nDisplayNames=,\nLayoutList=us,se\nUse=true\nVariantList=,\n"}},
			Detected{"se", "kde"}},
		{"kde no dbus", fakeEnv{vars: map[string]string{"XDG_CURRENT_DESKTOP": "KDE", "XDG_CONFIG_HOME": "/c"}, kde: -1, files: map[string]string{
			"/c/kxkbrc": "[Layout]\nLayoutList=es,us\nVariantList=,\n"}},
			Detected{"es", "kde"}},
		{"sway env", fakeEnv{vars: map[string]string{"XDG_CURRENT_DESKTOP": "sway", "XKB_DEFAULT_LAYOUT": "it,us"}}, Detected{"it", "env"}},
		{"x11", fakeEnv{vars: map[string]string{"XDG_SESSION_TYPE": "x11", "DISPLAY": ":0"}, cmds: map[string]string{
			"setxkbmap -query": "rules:      evdev\nmodel:      pc105\nlayout:     be\n"}},
			Detected{"be", "x11"}},
		{"localed xorg", fakeEnv{files: map[string]string{"/etc/X11/xorg.conf.d/00-keyboard.conf": `Section "InputClass"
        Identifier "system-keyboard"
        MatchIsKeyboard "on"
        Option "XkbLayout" "no"
EndSection
`}}, Detected{"no", "localectl"}},
		{"vconsole keymap", fakeEnv{files: map[string]string{"/etc/vconsole.conf": "KEYMAP=uk\nFONT=ter-v16n\n"}}, Detected{"gb", "localectl"}},
		{"vconsole fr_CH", fakeEnv{files: map[string]string{"/etc/vconsole.conf": "KEYMAP=fr_CH-latin1\n"}}, Detected{"ch-fr", "localectl"}},
		{"unknown", fakeEnv{vars: map[string]string{"XKB_DEFAULT_LAYOUT": "jp"}}, Detected{}},
	}
	for _, c := range cases {
		if got := Detect(c.env.env()); got != c.want {
			t.Errorf("%s: got %+v, want %+v", c.name, got, c.want)
		}
	}
}

func TestDetectHere(t *testing.T) {
	t.Logf("this machine: %+v", Detect(SystemEnv()))
}
