// Package steam identifies the game behind a "steam_app_<id>" window: its name
// (from the app manifests in every Steam library) and its library art.
package steam

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"
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

// GamePIDs lists our processes Steam launched for appid: Steam puts
// SteamAppId/SteamGameId in the environment of everything it starts for a game
// (native or Proton), and nothing else carries it.
func GamePIDs(appid string) []int {
	return gamePIDs("/proc", appid)
}

func gamePIDs(proc, appid string) []int {
	if appid == "" || appid == "0" {
		return nil
	}
	ents, _ := os.ReadDir(proc)
	self := os.Getpid()
	var out []int
	for _, e := range ents {
		pid, err := strconv.Atoi(e.Name())
		if err != nil || pid == self {
			continue
		}
		env, err := os.ReadFile(filepath.Join(proc, e.Name(), "environ"))
		if err != nil {
			continue // not ours, or gone
		}
		for _, kv := range bytes.Split(env, []byte{0}) {
			if string(kv) == "SteamAppId="+appid || string(kv) == "SteamGameId="+appid {
				out = append(out, pid)
				break
			}
		}
	}
	return out
}

// Close ends a game: SIGTERM to its processes, SIGKILL to what's left after 6 s.
func Close(appid string) (int, error) {
	pids := GamePIDs(appid)
	if len(pids) == 0 {
		return 0, fmt.Errorf("no running processes for app %s", appid)
	}
	for _, p := range pids {
		_ = syscall.Kill(p, syscall.SIGTERM)
	}
	go func() {
		for i := 0; i < 20; i++ {
			time.Sleep(300 * time.Millisecond)
			if len(GamePIDs(appid)) == 0 {
				return
			}
		}
		for _, p := range GamePIDs(appid) {
			_ = syscall.Kill(p, syscall.SIGKILL)
		}
	}()
	return len(pids), nil
}
