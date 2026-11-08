package configui

// The first-run setup in the configurator: its "done" flag, the board access
// rule, installed apps (for ready-made profiles) and the opt-in Vesktop flag.

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/beedywool-vibecoded/roundbar/internal/config"
	"github.com/beedywool-vibecoded/roundbar/internal/discord"
	"github.com/beedywool-vibecoded/roundbar/internal/icons"
	"github.com/beedywool-vibecoded/roundbar/internal/udev"
)

// uiState is ~/.config/roundbar/ui.json: configurator state that belongs to
// this PC, not to the board's config.
type uiState struct {
	SetupDone bool `json:"setupDone"`
}

func uiPath() string { return filepath.Join(config.Dir(), "ui.json") }

func loadUI() uiState {
	var s uiState
	if raw, err := os.ReadFile(uiPath()); err == nil {
		_ = json.Unmarshal(raw, &s)
	}
	return s
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func (s *server) getUI(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, loadUI())
}

// putUI merges the fields sent into ui.json.
func (s *server) putUI(w http.ResponseWriter, r *http.Request) {
	st := loadUI()
	if err := json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&st); err != nil {
		jsonErr(w, http.StatusBadRequest, "bad body", "")
		return
	}
	raw, _ := json.MarshalIndent(st, "", "  ")
	if err := os.MkdirAll(config.Dir(), 0o755); err != nil {
		jsonErr(w, http.StatusInternalServerError, err.Error(), "")
		return
	}
	tmp := uiPath() + ".tmp"
	err := os.WriteFile(tmp, append(raw, '\n'), 0o644)
	if err == nil {
		err = os.Rename(tmp, uiPath())
	}
	if err != nil {
		jsonErr(w, http.StatusInternalServerError, err.Error(), "")
		return
	}
	writeJSON(w, st)
}

// postUdev installs the udev rule like `roundbar udev`, through pkexec: the
// configurator has no terminal to ask for the password in.
func (s *server) postUdev(w http.ResponseWriter, r *http.Request) {
	if udevInstalled() {
		writeJSON(w, map[string]any{"ok": true, "already": true})
		return
	}
	if err := udev.Pkexec(); err != nil {
		var ee *exec.ExitError
		msg := err.Error()
		switch {
		case errors.Is(err, exec.ErrNotFound):
			msg = "pkexec isn't available: run `roundbar udev` in a terminal"
		case errors.As(err, &ee) && (ee.ExitCode() == 126 || ee.ExitCode() == 127):
			msg = "the password prompt was closed or refused"
		}
		jsonErr(w, http.StatusInternalServerError, msg, "")
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}

var udevInstalled = udev.Installed

// mangohudInstalled looks for the MangoHud binary, its Vulkan layer or the
// Flatpak layer that Steam's Flatpak uses.
var mangohudInstalled = func() bool {
	if _, err := exec.LookPath("mangohud"); err == nil {
		return true
	}
	home, _ := os.UserHomeDir()
	for _, pat := range []string{
		"/usr/share/vulkan/implicit_layer.d/MangoHud*.json",
		"/usr/share/vulkan/implicit_layer.d/mangohud*.json",
		"/var/lib/flatpak/runtime/org.freedesktop.Platform.VulkanLayer.MangoHud",
		filepath.Join(home, ".local/share/flatpak/runtime/org.freedesktop.Platform.VulkanLayer.MangoHud"),
	} {
		if m, _ := filepath.Glob(pat); len(m) > 0 {
			return true
		}
	}
	return false
}

// getApps: {apps: [{id, name, icon, categories, keys}], tools: {mangohud}}.
func (s *server) getApps(w http.ResponseWriter, r *http.Request) {
	apps := icons.Apps()
	for i := range apps {
		if apps[i].Icon != "" { // the browser asks /api/apps/{id}/icon
			apps[i].Icon = "/api/apps/" + apps[i].ID + "/icon"
		}
	}
	writeJSON(w, map[string]any{"apps": apps, "tools": map[string]bool{"mangohud": mangohudInstalled()}})
}

// getAppIcon serves an installed app's icon file (PNG, SVG or XPM).
func (s *server) getAppIcon(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	for _, a := range icons.Apps() {
		if a.ID != id {
			continue
		}
		f := icons.File(a.Icon)
		ext := strings.ToLower(filepath.Ext(f))
		if f == "" || (ext != ".png" && ext != ".svg" && ext != ".xpm") {
			break
		}
		w.Header().Set("Cache-Control", "private, max-age=3600")
		if ext == ".svg" {
			w.Header().Set("Content-Type", "image/svg+xml")
			w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'")
		}
		http.ServeFile(w, r, f)
		return
	}
	jsonErr(w, http.StatusNotFound, "no icon", "")
}

// getVesktop: {installs: [{kind, path, enabled}], port}.
func (s *server) getVesktop(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{"installs": vesktopInstalls(), "port": discord.CDPPort})
}

var vesktopInstalls = discord.VesktopInstalls

// postVesktop {"enable": bool} adds or removes the DevTools port flag in every
// Vesktop's flags file (the native one when none is found). Vesktop has to be
// restarted to pick it up.
func (s *server) postVesktop(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Enable *bool `json:"enable"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&body); err != nil || body.Enable == nil {
		jsonErr(w, http.StatusBadRequest, `send {"enable": true|false}`, "")
		return
	}
	paths := []string{}
	for _, in := range vesktopInstalls() {
		paths = append(paths, in.Path)
	}
	if len(paths) == 0 {
		paths = append(paths, discord.VesktopFlagsPath("native"))
	}
	changed := false
	for _, p := range paths {
		var ch bool
		var err error
		if *body.Enable {
			ch, err = discord.EnableVesktopFlag(p)
		} else {
			ch, err = discord.DisableVesktopFlag(p)
		}
		if err != nil {
			jsonErr(w, http.StatusInternalServerError, err.Error(), "")
			return
		}
		changed = changed || ch
	}
	writeJSON(w, map[string]any{"ok": true, "changed": changed, "installs": vesktopInstalls()})
}
