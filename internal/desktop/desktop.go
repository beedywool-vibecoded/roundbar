// Package desktop follows the desktop for the daemon: the media player
// (MPRIS on the session bus; Windows' media sessions), and through a Shell
// backend picked for the running desktop (GNOME extension, Hyprland, sway/i3,
// KWin, X11; Win32 on Windows) the focused app, windows, notifications and
// clipboard. Everything is event-driven: nothing here polls.
package desktop

import "sync"

type Focus struct {
	ID    string `json:"id"`    // lowercase app id without .desktop, or the window class
	Name  string `json:"name"`  // human name
	Class string `json:"class"` // lowercase WM_CLASS / app_id
}

type Media struct {
	Player  string `json:"player"`
	Title   string `json:"title"`
	Artist  string `json:"artist"`
	Album   string `json:"album"`
	Playing bool   `json:"playing"`
	CanNext bool   `json:"canNext"`
	CanPrev bool   `json:"canPrev"`
	ArtURL  string `json:"artUrl"`
}

type Window struct {
	ID    string `json:"id"`
	App   string `json:"app"`
	Title string `json:"title"`
	Icon  string `json:"icon"`
}

type Notification struct {
	ID    string `json:"id"`
	App   string `json:"app"`
	Title string `json:"title"`
	Body  string `json:"body"`
	Icon  string `json:"icon"`
}

// Info names the shell backend in use and what it can do.
type Info struct {
	Backend string `json:"backend"`
	Caps
}

type Desktop struct {
	platform // what only one OS has (D-Bus and MPRIS players on Linux)
	shell    Shell
	backend  string

	FocusChanged chan Focus
	MediaChanged chan struct{} // coalesced
	// Shell-side lists changed (coalesced). NotifNew carries the id of a new notification.
	WindowsChanged, ClipsChanged, NotifsChanged chan struct{}
	NotifNew                                    chan string
	// The keyboard layout may have changed: GNOME input sources were written in dconf,
	// or KDE switched layouts (coalesced).
	LayoutChanged chan struct{}

	mu sync.Mutex
}

func newDesktop() *Desktop {
	return &Desktop{FocusChanged: make(chan Focus, 4), MediaChanged: make(chan struct{}, 1),
		WindowsChanged: make(chan struct{}, 1), ClipsChanged: make(chan struct{}, 1), NotifsChanged: make(chan struct{}, 1),
		NotifNew: make(chan string, 4), LayoutChanged: make(chan struct{}, 1)}
}

// Info reports the backend and its capabilities.
func (d *Desktop) Info() Info { return Info{Backend: d.backend, Caps: d.shell.Caps()} }

// CurrentFocus asks the backend directly (used at startup).
func (d *Desktop) CurrentFocus() (Focus, bool) {
	f, err := d.shell.Focus()
	return f, err == nil
}

func (d *Desktop) Windows() ([]Window, error)             { return d.shell.Windows() }
func (d *Desktop) Activate(id string) error               { return d.shell.Activate(id) }
func (d *Desktop) Notifications() ([]Notification, error) { return d.shell.Notifications() }

// NotificationAction: "open" or "dismiss".
func (d *Desktop) NotificationAction(id, action string) error {
	return d.shell.NotificationAction(id, action)
}
func (d *Desktop) Clipboard() ([]string, error)   { return d.shell.Clipboard() }
func (d *Desktop) SetClipboard(text string) error { return d.shell.SetClipboard(text) }

// events from the backends

func (d *Desktop) focus(f Focus) {
	select {
	case d.FocusChanged <- f:
	default: // consumer is slow; it will get the next one
	}
}

func (d *Desktop) windows() { kick(d.WindowsChanged) }
func (d *Desktop) clips()   { kick(d.ClipsChanged) }

func (d *Desktop) notif(id string) {
	if id != "" {
		select {
		case d.NotifNew <- id:
		default:
		}
	}
	kick(d.NotifsChanged)
}

func kick(ch chan struct{}) {
	select {
	case ch <- struct{}{}:
	default:
	}
}

func (d *Desktop) notifyMedia() {
	select {
	case d.MediaChanged <- struct{}{}:
	default:
	}
}
