package keymap

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Env is what Detect looks at; tests replace it.
type Env struct {
	Getenv   func(string) string
	ReadFile func(string) ([]byte, error)
	Run      func(name string, args ...string) ([]byte, error)
	// KDEIndex returns the active entry of kxkbrc's LayoutList (D-Bus org.kde.keyboard), if known.
	KDEIndex func() (int, bool)
}

// SystemEnv reads the real environment. Commands get a short timeout; Detect only
// runs on connect and when the desktop says the layout changed.
func SystemEnv() Env {
	return Env{
		Getenv:   os.Getenv,
		ReadFile: os.ReadFile,
		Run: func(name string, args ...string) ([]byte, error) {
			if _, err := exec.LookPath(name); err != nil {
				return nil, err
			}
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			return exec.CommandContext(ctx, name, args...).Output()
		},
		KDEIndex: func() (int, bool) { return 0, false },
	}
}

// Detected is the PC's active layout: a keymap id and where it came from.
type Detected struct {
	ID     string `json:"detected"`
	Source string `json:"source"` // gnome, kde, env, x11, localectl
}

var gnomeSource = regexp.MustCompile(`\('(\w+)',\s*'([^']*)'\)`)

// Detect finds the active keyboard layout, trying in order: GNOME input sources
// (most recently used first), KDE's kxkbrc, XKB_DEFAULT_LAYOUT (wlroots compositors),
// setxkbmap on X11, and the system default from localed's files. It returns an empty
// ID when nothing maps to a layout we have.
func Detect(env Env) Detected {
	desktop := strings.ToUpper(env.Getenv("XDG_CURRENT_DESKTOP"))
	try := func(source, layout, variant string) (Detected, bool) {
		id := FromXKB(first(layout), first(variant))
		return Detected{id, source}, id != ""
	}
	if strings.Contains(desktop, "GNOME") || strings.Contains(desktop, "UNITY") || strings.Contains(desktop, "BUDGIE") {
		for _, key := range []string{"mru-sources", "sources"} {
			out, err := env.Run("gsettings", "get", "org.gnome.desktop.input-sources", key)
			if err != nil {
				break
			}
			if m := gnomeSource.FindAllStringSubmatch(string(out), -1); len(m) > 0 {
				// The first entry is the active one. An input method (ibus) types through
				// the layout of the first xkb source.
				for _, s := range m {
					if s[1] == "xkb" {
						l, v, _ := strings.Cut(s[2], "+")
						if d, ok := try("gnome", l, v); ok {
							return d
						}
						break
					}
				}
			}
		}
	}
	if strings.Contains(desktop, "KDE") {
		cfg := env.Getenv("XDG_CONFIG_HOME")
		if cfg == "" {
			cfg = filepath.Join(env.Getenv("HOME"), ".config")
		}
		if b, err := env.ReadFile(filepath.Join(cfg, "kxkbrc")); err == nil {
			ini := iniSection(string(b), "Layout")
			layouts, variants := strings.Split(ini["LayoutList"], ","), strings.Split(ini["VariantList"], ",")
			i, ok := env.KDEIndex()
			if !ok || i < 0 || i >= len(layouts) {
				i = 0
			}
			v := ""
			if i < len(variants) {
				v = variants[i]
			}
			if ini["Use"] != "false" {
				if d, ok := try("kde", layouts[i], v); ok {
					return d
				}
			}
		}
	}
	if d, ok := try("env", env.Getenv("XKB_DEFAULT_LAYOUT"), env.Getenv("XKB_DEFAULT_VARIANT")); ok {
		return d
	}
	if env.Getenv("XDG_SESSION_TYPE") == "x11" && env.Getenv("DISPLAY") != "" {
		if out, err := env.Run("setxkbmap", "-query"); err == nil {
			kv := colonFields(string(out))
			if d, ok := try("x11", kv["layout"], kv["variant"]); ok {
				return d
			}
		}
	}
	// What `localectl status` shows, read from the files systemd-localed keeps.
	if b, err := env.ReadFile("/etc/X11/xorg.conf.d/00-keyboard.conf"); err == nil {
		opt := regexp.MustCompile(`(?m)^\s*Option\s+"(XkbLayout|XkbVariant)"\s+"([^"]*)"`)
		kv := map[string]string{}
		for _, m := range opt.FindAllStringSubmatch(string(b), -1) {
			kv[m[1]] = m[2]
		}
		if d, ok := try("localectl", kv["XkbLayout"], kv["XkbVariant"]); ok {
			return d
		}
	}
	if b, err := env.ReadFile("/etc/vconsole.conf"); err == nil {
		kv := shellVars(string(b))
		if d, ok := try("localectl", kv["XKBLAYOUT"], kv["XKBVARIANT"]); ok {
			return d
		}
		// Console keymaps are named like de-latin1, fr, uk, sv-latin1.
		km, _, _ := strings.Cut(kv["KEYMAP"], "-")
		lv := [2]string{km, ""}
		if alias, ok := consoleAlias[km]; ok {
			lv = alias
		}
		if d, ok := try("localectl", lv[0], lv[1]); ok {
			return d
		}
	}
	return Detected{}
}

// Console keymap names that differ from the XKB layout (and variant).
var consoleAlias = map[string][2]string{"uk": {"gb", ""}, "sv": {"se", ""}, "sg": {"ch", ""}, "de_CH": {"ch", ""}, "fr_CH": {"ch", "fr"}}

func first(list string) string {
	f, _, _ := strings.Cut(list, ",")
	return strings.TrimSpace(f)
}

func iniSection(s, name string) map[string]string {
	out := map[string]string{}
	in := false
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[") {
			in = line == "["+name+"]"
			continue
		}
		if k, v, ok := strings.Cut(line, "="); ok && in {
			out[strings.TrimSpace(k)] = strings.TrimSpace(v)
		}
	}
	return out
}

func colonFields(s string) map[string]string {
	out := map[string]string{}
	for _, line := range strings.Split(s, "\n") {
		if k, v, ok := strings.Cut(line, ":"); ok {
			out[strings.TrimSpace(k)] = strings.TrimSpace(v)
		}
	}
	return out
}

func shellVars(s string) map[string]string {
	out := map[string]string{}
	for _, line := range strings.Split(s, "\n") {
		if k, v, ok := strings.Cut(strings.TrimSpace(line), "="); ok && !strings.HasPrefix(k, "#") {
			out[k] = strings.Trim(v, `"'`)
		}
	}
	return out
}
