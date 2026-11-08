package icons

import (
	"bufio"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// App is an installed application, from its desktop entry.
type App struct {
	ID         string   `json:"id"`   // lowercase desktop file id without .desktop (what focus reports)
	Name       string   `json:"name"` // Name=
	Icon       string   `json:"icon,omitempty"`
	Categories []string `json:"categories,omitempty"`
	// Other names the app's windows may report: StartupWMClass, the binary
	Keys []string `json:"keys,omitempty"`
}

// AppDirs are searched in order; a later entry with the same id replaces an
// earlier one (user entries override system ones). Replaceable in tests.
var AppDirs = func() []string {
	home, _ := os.UserHomeDir()
	return []string{
		"/usr/share/applications",
		"/usr/local/share/applications",
		"/var/lib/flatpak/exports/share/applications",
		filepath.Join(home, ".local/share/flatpak/exports/share/applications"),
		filepath.Join(home, ".local/share/applications"),
	}
}

// Apps lists the applications a user can launch: desktop entries of type
// Application that aren't hidden, sorted by name.
func Apps() []App {
	byID := map[string]App{}
	for _, dir := range AppDirs() {
		files, _ := filepath.Glob(filepath.Join(dir, "*.desktop"))
		for _, f := range files {
			a, show := parseApp(f)
			if !show {
				delete(byID, a.ID) // NoDisplay / Hidden in a later dir hides it
				continue
			}
			byID[a.ID] = a
		}
	}
	out := make([]App, 0, len(byID))
	for _, a := range byID {
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool {
		if li, lj := strings.ToLower(out[i].Name), strings.ToLower(out[j].Name); li != lj {
			return li < lj
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func parseApp(path string) (App, bool) {
	a := App{ID: strings.ToLower(strings.TrimSuffix(filepath.Base(path), ".desktop"))}
	f, err := os.Open(path)
	if err != nil {
		return a, false
	}
	defer f.Close()
	typ, show := "", true
	inMain := false
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "[") {
			inMain = line == "[Desktop Entry]"
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !inMain || !ok {
			continue
		}
		switch strings.TrimSpace(k) {
		case "Type":
			typ = v
		case "Name":
			a.Name = v
		case "Icon":
			a.Icon = v
		case "NoDisplay", "Hidden":
			if v == "true" {
				show = false
			}
		case "Categories":
			for _, c := range strings.Split(v, ";") {
				if c = strings.TrimSpace(c); c != "" {
					a.Categories = append(a.Categories, c)
				}
			}
		case "StartupWMClass":
			addKey(&a, v)
		case "Exec":
			if fields := strings.Fields(v); len(fields) > 0 && fields[0] != "env" && fields[0] != "flatpak" {
				addKey(&a, filepath.Base(fields[0]))
			}
		}
	}
	return a, show && typ == "Application" && a.Name != ""
}

func addKey(a *App, k string) {
	k = strings.ToLower(strings.TrimSpace(k))
	if k == "" || k == "." || k == a.ID {
		return
	}
	for _, x := range a.Keys {
		if x == k {
			return
		}
	}
	a.Keys = append(a.Keys, k)
}

// File resolves an icon name (or absolute path) to an image file in the icon
// themes, or "".
func File(name string) string {
	if name == "" {
		return ""
	}
	return find(name)
}
