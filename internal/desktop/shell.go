package desktop

import (
	"errors"
	"slices"
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
	BackendWindows = "windows" // Win32 window events, clipboard listener, media sessions
)
