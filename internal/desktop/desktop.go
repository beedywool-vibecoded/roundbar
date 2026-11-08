// Package desktop follows the desktop for the daemon: MPRIS media players on
// the session bus, and through a Shell backend picked for the running desktop
// (GNOME extension, Hyprland, sway/i3, KWin, X11) the focused app, windows,
// notifications and clipboard. Everything is event-driven: nothing here polls.
package desktop

import (
	"log"
	"os"
	"strings"
	"sync"

	"github.com/godbus/dbus/v5"
)

const (
	mprisPath  = "/org/mpris/MediaPlayer2"
	mprisIface = "org.mpris.MediaPlayer2.Player"
	mprisPre   = "org.mpris.MediaPlayer2."

	dconfIface  = "ca.desrt.dconf.Writer"
	inputPrefix = "/org/gnome/desktop/input-sources/"
	kdeKbdName  = "org.kde.keyboard"
	kdeKbdPath  = "/Layouts"
	kdeKbdIface = "org.kde.KeyboardLayouts"
)

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

type player struct {
	name   string // bus name
	owner  string // unique name, signals carry it as sender
	ident  string
	status string
	meta   map[string]dbus.Variant
	next   bool
	prev   bool
	seq    int // last time it started playing
}

type Desktop struct {
	conn    *dbus.Conn
	shell   Shell
	backend string

	FocusChanged chan Focus
	MediaChanged chan struct{} // coalesced
	// Shell-side lists changed (coalesced). NotifNew carries the id of a new notification.
	WindowsChanged, ClipsChanged, NotifsChanged chan struct{}
	NotifNew                                    chan string
	// The keyboard layout may have changed: GNOME input sources were written in dconf,
	// or KDE switched layouts (coalesced).
	LayoutChanged chan struct{}

	mu      sync.Mutex
	players map[string]*player // by owner
	seq     int
}

func Connect() (*Desktop, error) {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return nil, err
	}
	d := &Desktop{conn: conn, FocusChanged: make(chan Focus, 4), MediaChanged: make(chan struct{}, 1), players: map[string]*player{},
		WindowsChanged: make(chan struct{}, 1), ClipsChanged: make(chan struct{}, 1), NotifsChanged: make(chan struct{}, 1),
		NotifNew: make(chan string, 4), LayoutChanged: make(chan struct{}, 1)}
	if err := conn.AddMatchSignal(dbus.WithMatchInterface("org.freedesktop.DBus.Properties"), dbus.WithMatchMember("PropertiesChanged"),
		dbus.WithMatchObjectPath(mprisPath), dbus.WithMatchArg(0, mprisIface)); err != nil {
		return nil, err
	}
	if err := conn.AddMatchSignal(dbus.WithMatchSender("org.freedesktop.DBus"), dbus.WithMatchMember("NameOwnerChanged"),
		dbus.WithMatchArg0Namespace("org.mpris.MediaPlayer2")); err != nil {
		return nil, err
	}
	// Best effort: neither is required.
	_ = conn.AddMatchSignal(dbus.WithMatchInterface(dconfIface), dbus.WithMatchMember("Notify"))
	_ = conn.AddMatchSignal(dbus.WithMatchInterface(kdeKbdIface), dbus.WithMatchObjectPath(kdeKbdPath))
	sig := make(chan *dbus.Signal, 16)
	conn.Signal(sig)
	go d.loop(sig)

	var names []string
	if err := conn.BusObject().Call("org.freedesktop.DBus.ListNames", 0).Store(&names); err == nil {
		for _, n := range names {
			if strings.HasPrefix(n, mprisPre) {
				d.addPlayer(n)
			}
		}
	}
	d.backend = pick(os.Getenv)
	d.shell = d.startShell(d.backend)
	return d, nil
}

// startShell starts the backend for focus and windows, then fills in
// notifications and clipboard from generic sources where it has none.
func (d *Desktop) startShell(backend string) Shell {
	var win Shell = none{}
	var err error
	switch backend {
	case BackendGnome:
		g, err := newGnome(d.conn, d)
		if err != nil {
			log.Printf("desktop: gnome: %v", err)
			return none{}
		}
		return g
	case BackendKDE:
		win, err = newKDE(d.conn, d)
	case BackendWayland:
		win, err = newWayland(d, true)
		if err != nil && os.Getenv("XDG_CURRENT_DESKTOP") == "" {
			log.Printf("desktop: wayland: %v; trying the GNOME extension", err)
			d.backend = BackendGnome
			return d.startShell(BackendGnome)
		}
	case BackendX11:
		win, err = newX11(d)
	case BackendNone:
	default:
		log.Printf("desktop: unknown backend %q", backend)
	}
	if err != nil {
		log.Printf("desktop: %s: %v", backend, err)
		win = none{}
	}
	m := mix{win: win, notif: win, clip: win}
	if !win.Caps().Notifications {
		if n, err := newNotifyMonitor(d.conn, d, win); err != nil {
			log.Printf("desktop: notifications: %v", err)
		} else {
			m.notif = n
		}
	}
	if k, ok := win.(*kde); ok {
		if c := k.klipper(); c != nil {
			m.clip = c
		}
	}
	if !m.clip.Caps().Clipboard && backend == BackendKDE && os.Getenv("WAYLAND_DISPLAY") != "" {
		if c, err := newWayland(d, false); err == nil && c.Caps().Clipboard { // KWin speaks data control
			m.clip = c
		} else if c != nil {
			c.Close()
		}
	}
	return m
}

// KDELayoutIndex asks KDE which entry of its layout list is active.
func (d *Desktop) KDELayoutIndex() (int, bool) {
	var i uint32
	err := d.conn.Object(kdeKbdName, kdeKbdPath).Call(kdeKbdIface+".getLayout", dbus.FlagNoAutoStart).Store(&i)
	return int(i), err == nil
}

func (d *Desktop) Close() {
	d.shell.Close()
	d.conn.Close()
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

func (d *Desktop) loop(sig chan *dbus.Signal) {
	for s := range sig {
		switch {
		case s.Name == dconfIface+".Notify" && len(s.Body) >= 2:
			prefix, _ := s.Body[0].(string)
			changes, _ := s.Body[1].([]string)
			if len(changes) == 0 {
				changes = []string{""}
			}
			for _, c := range changes {
				if p := prefix + c; strings.HasPrefix(p, inputPrefix) || strings.HasPrefix(inputPrefix, p) {
					kick(d.LayoutChanged)
					break
				}
			}
		case strings.HasPrefix(s.Name, kdeKbdIface+"."):
			kick(d.LayoutChanged)
		case s.Name == "org.freedesktop.DBus.NameOwnerChanged" && len(s.Body) == 3:
			name, _ := s.Body[0].(string)
			oldOwner, _ := s.Body[1].(string)
			newOwner, _ := s.Body[2].(string)
			if !strings.HasPrefix(name, mprisPre) {
				continue
			}
			if oldOwner != "" {
				d.mu.Lock()
				delete(d.players, oldOwner)
				d.mu.Unlock()
				d.notifyMedia()
			}
			if newOwner != "" {
				go d.addPlayer(name)
			}
		case s.Name == "org.freedesktop.DBus.Properties.PropertiesChanged" && len(s.Body) >= 2 && s.Path == mprisPath:
			changed, _ := s.Body[1].(map[string]dbus.Variant)
			d.mu.Lock()
			p := d.players[s.Sender]
			if p != nil {
				d.apply(p, changed)
			}
			d.mu.Unlock()
			if p != nil {
				d.notifyMedia()
			}
		}
	}
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

func (d *Desktop) addPlayer(name string) {
	var owner string
	if err := d.conn.BusObject().Call("org.freedesktop.DBus.GetNameOwner", 0, name).Store(&owner); err != nil {
		return
	}
	obj := d.conn.Object(name, mprisPath)
	var props map[string]dbus.Variant
	if err := obj.Call("org.freedesktop.DBus.Properties.GetAll", 0, mprisIface).Store(&props); err != nil {
		return
	}
	ident := strings.TrimPrefix(name, mprisPre)
	if v, err := obj.GetProperty("org.mpris.MediaPlayer2.Identity"); err == nil {
		if s, ok := v.Value().(string); ok {
			ident = s
		}
	}
	d.mu.Lock()
	p := &player{name: name, owner: owner, ident: ident}
	d.apply(p, props)
	d.players[owner] = p
	d.mu.Unlock()
	d.notifyMedia()
}

func (d *Desktop) apply(p *player, props map[string]dbus.Variant) {
	if v, ok := props["PlaybackStatus"]; ok {
		s, _ := v.Value().(string)
		if s == "Playing" && p.status != "Playing" {
			d.seq++
			p.seq = d.seq
		}
		p.status = s
	}
	if v, ok := props["Metadata"]; ok {
		p.meta, _ = v.Value().(map[string]dbus.Variant)
	}
	if v, ok := props["CanGoNext"]; ok {
		p.next, _ = v.Value().(bool)
	}
	if v, ok := props["CanGoPrevious"]; ok {
		p.prev, _ = v.Value().(bool)
	}
}

func metaString(m map[string]dbus.Variant, k string) string {
	v, ok := m[k]
	if !ok {
		return ""
	}
	switch x := v.Value().(type) {
	case string:
		return x
	case []string:
		return strings.Join(x, ", ")
	}
	return ""
}

// Media returns the player that is playing (most recently started first),
// otherwise the one that played last. ok=false when there is no player.
func (d *Desktop) Media() (Media, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	var best *player
	for _, p := range d.players {
		switch {
		case best == nil:
			best = p
		case (p.status == "Playing") != (best.status == "Playing"):
			if p.status == "Playing" {
				best = p
			}
		case p.seq > best.seq:
			best = p
		}
	}
	if best == nil {
		return Media{}, false
	}
	return Media{
		Player:  best.ident,
		Title:   metaString(best.meta, "xesam:title"),
		Artist:  metaString(best.meta, "xesam:artist"),
		Album:   metaString(best.meta, "xesam:album"),
		Playing: best.status == "Playing",
		CanNext: best.next,
		CanPrev: best.prev,
		ArtURL:  metaString(best.meta, "mpris:artUrl"),
	}, true
}
