// Package steam identifies the game behind a "steam_app_<id>" window: its name
// (from the app manifests in every Steam library) and its library art.
package steam

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var appRe = regexp.MustCompile(`^steam_app_(\d+)$`)

// AppID returns the Steam app id of a window class like "steam_app_1091500".
func AppID(class string) string {
	if m := appRe.FindStringSubmatch(strings.ToLower(class)); m != nil {
		return m[1]
	}
	return ""
}

func root() string {
	home, _ := os.UserHomeDir()
	for _, p := range []string{".local/share/Steam", ".steam/steam", ".var/app/com.valvesoftware.Steam/.local/share/Steam"} {
		if _, err := os.Stat(filepath.Join(home, p, "steamapps")); err == nil {
			return filepath.Join(home, p)
		}
	}
	return filepath.Join(home, ".local/share/Steam")
}

var pathRe = regexp.MustCompile(`"path"\s+"([^"]+)"`)
var nameRe = regexp.MustCompile(`"name"\s+"([^"]+)"`)

// libraries lists every Steam library folder (libraryfolders.vdf).
func libraries() []string {
	r := root()
	libs := []string{r}
	raw, err := os.ReadFile(filepath.Join(r, "steamapps", "libraryfolders.vdf"))
	if err == nil {
		for _, m := range pathRe.FindAllStringSubmatch(string(raw), -1) {
			if m[1] != r {
				libs = append(libs, m[1])
			}
		}
	}
	return libs
}

// Name returns the game's name from its appmanifest, or "".
func Name(appid string) string {
	for _, lib := range libraries() {
		raw, err := os.ReadFile(filepath.Join(lib, "steamapps", "appmanifest_"+appid+".acf"))
		if err != nil {
			continue
		}
		if m := nameRe.FindStringSubmatch(string(raw)); m != nil {
			return m[1]
		}
	}
	return ""
}

// Art returns the best library image for a background: the wide hero art
// first, then the header, then the vertical capsule. Steam keeps them in
// per-version hashed subfolders of appcache/librarycache/<appid>.
func Art(appid string) string {
	dir := filepath.Join(root(), "appcache", "librarycache", appid)
	for _, name := range []string{"library_hero.jpg", "header.jpg", "library_600x900.jpg", "library_capsule.jpg"} {
		if m, _ := filepath.Glob(filepath.Join(dir, name)); len(m) > 0 {
			return m[0]
		}
		if m, _ := filepath.Glob(filepath.Join(dir, "*", name)); len(m) > 0 {
			return m[0]
		}
	}
	if m, _ := filepath.Glob(filepath.Join(dir, "*.jpg")); len(m) > 0 {
		return m[0]
	}
	return ""
}
