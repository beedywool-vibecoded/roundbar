package desktop

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Desktop entries turn a window class into what GNOME reports: the app id
// (desktop file name), its human name and icon. Loaded on first use, reloaded
// only when an applications directory changed since.

type app struct{ id, name, icon string }

var apps struct {
	sync.Mutex
	strong, weak map[string]app // file id / StartupWMClass; last part of a reverse-DNS id
	stamp        time.Time
}

func appDirs() []string {
	home, _ := os.UserHomeDir()
	data := os.Getenv("XDG_DATA_HOME")
	if data == "" {
		data = filepath.Join(home, ".local/share")
	}
	sys := os.Getenv("XDG_DATA_DIRS")
	if sys == "" {
		sys = "/usr/local/share:/usr/share"
	}
	// first wins: user entries override system ones
	dirs := []string{filepath.Join(data, "applications")}
	for _, d := range strings.Split(sys, ":") {
		if d != "" {
			dirs = append(dirs, filepath.Join(d, "applications"))
		}
	}
	return append(dirs, filepath.Join(data, "flatpak/exports/share/applications"), "/var/lib/flatpak/exports/share/applications")
}

func appsStamp(dirs []string) time.Time {
	var t time.Time
	for _, d := range dirs {
		if st, err := os.Stat(d); err == nil && st.ModTime().After(t) {
			t = st.ModTime()
		}
	}
	return t
}

func loadApps(dirs []string) {
	apps.strong, apps.weak = map[string]app{}, map[string]app{}
	add := func(m map[string]app, k string, a app) {
		k = strings.ToLower(k)
		if _, ok := m[k]; k != "" && !ok {
			m[k] = a
		}
	}
	for _, dir := range dirs {
		files, _ := filepath.Glob(filepath.Join(dir, "*.desktop"))
		for _, f := range files {
			a, wmclass, ok := parseEntry(f)
			if !ok {
				continue
			}
			add(apps.strong, a.id, a)
			add(apps.strong, wmclass, a)
			if i := strings.LastIndexByte(a.id, '.'); i >= 0 {
				add(apps.weak, a.id[i+1:], a)
			}
		}
	}
}

func parseEntry(path string) (a app, wmclass string, ok bool) {
	f, err := os.Open(path)
	if err != nil {
		return a, "", false
	}
	defer f.Close()
	a.id = strings.TrimSuffix(filepath.Base(path), ".desktop")
	inMain := false
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "[") {
			inMain = line == "[Desktop Entry]"
			continue
		}
		k, v, found := strings.Cut(line, "=")
		if !inMain || !found {
			continue
		}
		switch strings.TrimSpace(k) {
		case "Name":
			a.name = strings.TrimSpace(v)
		case "Icon":
			a.icon = strings.TrimSpace(v)
		case "StartupWMClass":
			wmclass = strings.TrimSpace(v)
		case "Hidden":
			if strings.TrimSpace(v) == "true" {
				return a, "", false
			}
		}
	}
	return a, wmclass, true
}

// lookupApp finds the desktop entry for the first key (desktop id, window
// class, instance…) that names one.
func lookupApp(keys ...string) (app, bool) {
	apps.Lock()
	defer apps.Unlock()
	dirs := appDirs()
	if apps.strong == nil {
		apps.stamp = appsStamp(dirs)
		loadApps(dirs)
	}
	for try := 0; try < 2; try++ {
		for _, m := range []map[string]app{apps.strong, apps.weak} {
			for _, k := range keys {
				if a, ok := m[strings.ToLower(strings.TrimSuffix(k, ".desktop"))]; ok && k != "" {
					return a, true
				}
			}
		}
		// a miss: maybe the app was installed since we loaded
		if st := appsStamp(dirs); try == 0 && st.After(apps.stamp) {
			apps.stamp = st
			loadApps(dirs)
			continue
		}
		break
	}
	return app{}, false
}

// focusFor builds a Focus the way the GNOME extension does: ID is the
// lowercase desktop id when known (else the class), Class the lowercase
// window class, Name the app's name (else the title).
func focusFor(desktopID, class, instance, title string) Focus {
	cls := strings.ToLower(class)
	f := Focus{ID: cls, Class: cls, Name: title}
	if a, ok := lookupApp(desktopID, class, instance); ok {
		f.ID = strings.ToLower(a.id)
		if a.name != "" {
			f.Name = a.name
		}
	} else if desktopID != "" {
		f.ID = strings.ToLower(strings.TrimSuffix(desktopID, ".desktop"))
	}
	if f.Name == "" {
		f.Name = class
	}
	return f
}

// windowFor builds a Window entry: App is the desktop id (else the class).
func windowFor(id, desktopID, class, instance, title string) Window {
	w := Window{ID: id, App: class, Title: title}
	if a, ok := lookupApp(desktopID, class, instance); ok {
		w.App, w.Icon = a.id, a.icon
	} else if desktopID != "" {
		w.App = strings.TrimSuffix(desktopID, ".desktop")
	}
	return w
}
