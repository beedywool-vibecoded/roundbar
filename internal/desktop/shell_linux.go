package desktop

import (
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// pick chooses the backend from the session environment. ROUNDBAR_DESKTOP
// overrides it.
func pick(getenv func(string) string) string {
	if b := strings.ToLower(getenv("ROUNDBAR_DESKTOP")); b != "" {
		return b
	}
	cur := strings.Split(strings.ToLower(getenv("XDG_CURRENT_DESKTOP")), ":")
	has := func(s string) bool { return slices.Contains(cur, s) }
	switch {
	case has("kde"):
		return BackendKDE
	case has("gnome") && !has("budgie") && !has("gnome-flashback"):
		return BackendGnome
	case getenv("WAYLAND_DISPLAY") != "", slices.ContainsFunc(cur, func(s string) bool { return slices.Contains(wlrDesktops, s) }):
		return BackendWayland
	case getenv("DISPLAY") != "":
		return BackendX11
	case cur[0] == "":
		return BackendGnome // nothing to go on (early service start): what roundbar always assumed
	}
	return BackendNone
}

// wlrDesktops are Wayland-only sessions, picked even when WAYLAND_DISPLAY
// didn't reach the systemd user environment.
var wlrDesktops = []string{"hyprland", "sway", "river", "niri", "labwc", "wayfire", "cosmic"}

// Detect names the backend Connect would pick for this session.
func Detect() string { return pick(os.Getenv) }

// runtimeDir is $XDG_RUNTIME_DIR, or the usual /run/user/<uid>.
func runtimeDir() string {
	if d := os.Getenv("XDG_RUNTIME_DIR"); d != "" {
		return d
	}
	return filepath.Join("/run/user", strconv.Itoa(os.Getuid()))
}
