package icons

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Desktop entries tell us the icon an app really uses (including icon theme
// overrides set in its .desktop file); PipeWire's application.icon_name is often
// generic (Electron apps report "chromium" or nothing).
var (
	deskOnce  sync.Once
	deskIndex map[string]string // lowercase key -> Icon=
)

func loadDesktop() {
	deskIndex = map[string]string{}
	home, _ := os.UserHomeDir()
	dirs := []string{ // later dirs win: user entries override system ones
		"/usr/share/applications",
		"/var/lib/flatpak/exports/share/applications",
		filepath.Join(home, ".local/share/flatpak/exports/share/applications"),
		filepath.Join(home, ".local/share/applications"),
	}
	for _, dir := range dirs {
		files, _ := filepath.Glob(filepath.Join(dir, "*.desktop"))
		for _, f := range files {
			icon, keys := parseDesktop(f)
			if icon == "" {
				continue
			}
			for _, k := range keys {
				if k != "" {
					deskIndex[strings.ToLower(k)] = icon
				}
			}
		}
	}
}

func parseDesktop(path string) (string, []string) {
	f, err := os.Open(path)
	if err != nil {
		return "", nil
	}
	defer f.Close()
	keys := []string{strings.TrimSuffix(filepath.Base(path), ".desktop")}
	icon := ""
	inMain := false
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "[") {
			inMain = line == "[Desktop Entry]"
			continue
		}
		if !inMain {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		switch k {
		case "Icon":
			icon = v
		case "Name", "StartupWMClass":
			keys = append(keys, v)
		case "Exec":
			if fields := strings.Fields(v); len(fields) > 0 {
				exe := fields[0]
				if exe == "env" || exe == "flatpak" { // skip wrappers, take the last plain word
					for _, w := range fields[1:] {
						if !strings.Contains(w, "=") && !strings.HasPrefix(w, "-") {
							exe = w
						}
					}
				}
				keys = append(keys, filepath.Base(exe))
			}
		}
	}
	return icon, keys
}

// ForApp returns the icon from the first candidate (binary, app name, window
// class…) that matches a desktop entry, or "".
func ForApp(candidates ...string) string {
	deskOnce.Do(loadDesktop)
	for _, c := range candidates {
		if c == "" {
			continue
		}
		if icon, ok := deskIndex[strings.ToLower(c)]; ok {
			return icon
		}
	}
	return ""
}
