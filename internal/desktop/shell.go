package desktop

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// ErrUnsupported is returned by shell features the desktop can't provide.
// Callers treat it as an empty list.
var ErrUnsupported = errors.New("not supported on this desktop")

// Caps says which shell features the active backend provides.
type Caps struct {
	Focus         bool `json:"focus"`
	Windows       bool `json:"windows"`
	Notifications bool `json:"notifications"`
	Clipboard     bool `json:"clipboard"`
}

// Shell is a desktop backend for what has no common D-Bus API: the focused
// app, open windows, notifications and clipboard history. Backends push
// changes through the events they were created with and return
// ErrUnsupported for what they can't do.
type Shell interface {
	Caps() Caps
	Focus() (Focus, error)
	Windows() ([]Window, error)
	Activate(id string) error
	Notifications() ([]Notification, error)
	NotificationAction(id, action string) error
	Clipboard() ([]string, error)
	SetClipboard(text string) error
	Close()
}

// events is how backends report changes (Desktop forwards them to its channels).
type events interface {
	focus(Focus)
	windows()
	clips()
	notif(id string) // id of a new notification, "" for any other change
}

// none supports nothing; backends embed it and override what they do.
type none struct{}

func (none) Caps() Caps                              { return Caps{} }
func (none) Focus() (Focus, error)                   { return Focus{}, ErrUnsupported }
func (none) Windows() ([]Window, error)              { return nil, ErrUnsupported }
func (none) Activate(string) error                   { return ErrUnsupported }
func (none) Notifications() ([]Notification, error)  { return nil, ErrUnsupported }
func (none) NotificationAction(string, string) error { return ErrUnsupported }
func (none) Clipboard() ([]string, error)            { return nil, ErrUnsupported }
func (none) SetClipboard(string) error               { return ErrUnsupported }
func (none) Close()                                  {}

// mix takes focus and windows, notifications and clipboard from different backends.
type mix struct{ win, notif, clip Shell }

func (m mix) Caps() Caps {
	w := m.win.Caps()
	return Caps{Focus: w.Focus, Windows: w.Windows, Notifications: m.notif.Caps().Notifications, Clipboard: m.clip.Caps().Clipboard}
}
func (m mix) Focus() (Focus, error)                  { return m.win.Focus() }
func (m mix) Windows() ([]Window, error)             { return m.win.Windows() }
func (m mix) Activate(id string) error               { return m.win.Activate(id) }
func (m mix) Notifications() ([]Notification, error) { return m.notif.Notifications() }
func (m mix) NotificationAction(id, action string) error {
	return m.notif.NotificationAction(id, action)
}
func (m mix) Clipboard() ([]string, error)   { return m.clip.Clipboard() }
func (m mix) SetClipboard(text string) error { return m.clip.SetClipboard(text) }

func (m mix) Close() {
	var done []Shell
	for _, s := range []Shell{m.win, m.notif, m.clip} {
		if !slices.Contains(done, s) {
			s.Close()
			done = append(done, s)
		}
	}
}

// Backend names, as reported in the daemon status.
const (
	BackendGnome   = "gnome"   // our GNOME Shell extension
	BackendKDE     = "kde"     // KWin script + Klipper
	BackendWayland = "wayland" // foreign-toplevel and data-control protocols (wlroots, Hyprland, niri…)
	BackendX11     = "x11"     // EWMH root window properties
	BackendNone    = "none"
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
