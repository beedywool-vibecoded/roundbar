// Package steam identifies the game behind a "steam_app_<id>" window: its name
// (from the app manifests in every Steam library) and its library art.
package steam

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

var appRe = regexp.MustCompile(`^steam_app_(\d+)$`)

// AppID returns the Steam app id of a window class like "steam_app_1091500".
func AppID(class string) string {
	if m := appRe.FindStringSubmatch(strings.ToLower(class)); m != nil {
		return m[1]
	}
	return ""
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
			if p := vdfUnescape(m[1]); !samePath(p, r) {
				libs = append(libs, p)
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
			return vdfUnescape(m[1])
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

// gamePIDs lists our processes under proc (a procfs) that Steam launched for
// appid: Steam puts SteamAppId/SteamGameId in the environment of everything
// it starts for a game (native or Proton), and nothing else carries it.
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

// samePath compares library paths, ignoring case and separators on Windows-style paths.
func samePath(a, b string) bool {
	if a == b {
		return true
	}
	norm := func(s string) string { return strings.TrimRight(strings.ToLower(strings.ReplaceAll(s, `\`, "/")), "/") }
	return strings.Contains(a+b, `\`) && norm(a) == norm(b)
}

// vdfUnescape undoes VDF string escapes (Windows paths come as "D:\\SteamLibrary").
func vdfUnescape(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			i++
			switch s[i] {
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			default:
				b.WriteByte(s[i])
			}
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

var installRe = regexp.MustCompile(`"installdir"\s+"([^"]+)"`)
var appidRe = regexp.MustCompile(`"appid"\s+"(\d+)"`)

// SplitGamePath splits an executable path inside a Steam library into the
// library folder and the game's install directory name:
// D:\SteamLibrary\steamapps\common\Hades\x64\Hades.exe -> D:\SteamLibrary, Hades.
// Both separators are accepted; "steamapps\common" matches in any case.
func SplitGamePath(exe string) (lib, installDir string, ok bool) {
	p := strings.ReplaceAll(exe, `\`, "/")
	const mid = "/steamapps/common/"
	i := strings.Index(strings.ToLower(p), mid)
	if i <= 0 {
		return "", "", false
	}
	dir, _, found := strings.Cut(p[i+len(mid):], "/")
	if !found || dir == "" {
		return "", "", false
	}
	return exe[:i], dir, true
}

// manifestApp reads an appmanifest's app id and install directory.
func manifestApp(raw []byte) (appid, installDir string) {
	if m := appidRe.FindSubmatch(raw); m != nil {
		appid = string(m[1])
	}
	if m := installRe.FindSubmatch(raw); m != nil {
		installDir = vdfUnescape(string(m[1]))
	}
	return
}

// AppForInstallDir finds the app whose manifest in lib names installDir (case-insensitive).
func AppForInstallDir(lib, installDir string) string {
	files, _ := filepath.Glob(filepath.Join(lib, "steamapps", "appmanifest_*.acf"))
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		if id, dir := manifestApp(raw); id != "" && strings.EqualFold(dir, installDir) {
			return id
		}
	}
	return ""
}

var exeApps sync.Map // exe path -> app id ("" = not a Steam game)

// AppForExe is the Steam app an executable belongs to, by where it lives
// (steamapps/common/<installdir> of a library whose manifest names that
// directory), or "". Windows has no steam_app_<id> window classes, so this is
// how games are recognized there. Cached per path.
func AppForExe(exe string) string {
	if v, ok := exeApps.Load(exe); ok {
		return v.(string)
	}
	id := ""
	if lib, dir, ok := SplitGamePath(exe); ok {
		id = AppForInstallDir(lib, dir)
	}
	if id != "" || !strings.Contains(strings.ToLower(exe), "steamapps") {
		exeApps.Store(exe, id) // a game whose manifest is missing may get one later: retry those
	}
	return id
}

// InstallPath is the folder a game is installed in ("" if no manifest names it).
func InstallPath(appid string) string {
	for _, lib := range libraries() {
		raw, err := os.ReadFile(filepath.Join(lib, "steamapps", "appmanifest_"+appid+".acf"))
		if err != nil {
			continue
		}
		if _, dir := manifestApp(raw); dir != "" {
			return filepath.Join(lib, "steamapps", "common", dir)
		}
	}
	return ""
}
