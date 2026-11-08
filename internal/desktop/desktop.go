// Package desktop listens on the session bus for the focused app (published by the
// roundbar GNOME Shell extension) and for MPRIS media players. Everything is
// signal-driven: nothing here polls.
package desktop

import (
	"strings"
	"sync"

	"github.com/godbus/dbus/v5"
)

const (
	focusName  = "dev.roundbar.Focus"
	focusPath  = "/dev/roundbar/Focus"
	focusIface = "dev.roundbar.Focus"
	mprisPath  = "/org/mpris/MediaPlayer2"
	mprisIface = "org.mpris.MediaPlayer2.Player"
	mprisPre   = "org.mpris.MediaPlayer2."
)

type Focus struct {
	ID    string `json:"id"`    // lowercase app id without .desktop, or the window class
	Name  string `json:"name"`  // human name
	Class string `json:"class"` // lowercase WM_CLASS
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
	conn *dbus.Conn

	FocusChanged chan Focus
	MediaChanged chan struct{} // coalesced
	// Shell-side lists changed (coalesced). NotifNew carries the id of a new notification.
	WindowsChanged, ClipsChanged, NotifsChanged chan struct{}
	NotifNew                                    chan string

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
		NotifNew: make(chan string, 4)}
	if err := conn.AddMatchSignal(dbus.WithMatchInterface(focusIface), dbus.WithMatchObjectPath(focusPath)); err != nil {
		return nil, err
	}
	if err := conn.AddMatchSignal(dbus.WithMatchInterface("org.freedesktop.DBus.Properties"), dbus.WithMatchMember("PropertiesChanged"),
		dbus.WithMatchObjectPath(mprisPath), dbus.WithMatchArg(0, mprisIface)); err != nil {
		return nil, err
	}
	if err := conn.AddMatchSignal(dbus.WithMatchSender("org.freedesktop.DBus"), dbus.WithMatchMember("NameOwnerChanged"),
		dbus.WithMatchArg0Namespace("org.mpris.MediaPlayer2")); err != nil {
		return nil, err
	}
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
	return d, nil
}

func (d *Desktop) Close() { d.conn.Close() }

// CurrentFocus asks the extension directly (used at startup).
func (d *Desktop) CurrentFocus() (Focus, bool) {
	var f Focus
	err := d.conn.Object(focusName, focusPath).Call(focusIface+".Get", 0).Store(&f.ID, &f.Name, &f.Class)
	return f, err == nil
}

func (d *Desktop) loop(sig chan *dbus.Signal) {
	for s := range sig {
		switch {
		case s.Name == focusIface+".Changed" && len(s.Body) >= 3:
			f := Focus{}
			f.ID, _ = s.Body[0].(string)
			f.Name, _ = s.Body[1].(string)
			f.Class, _ = s.Body[2].(string)
			select {
			case d.FocusChanged <- f:
			default: // consumer is slow; it will get the next one
			}
		case s.Name == focusIface+".WindowsChanged":
			kick(d.WindowsChanged)
		case s.Name == focusIface+".ClipboardChanged":
			kick(d.ClipsChanged)
		case s.Name == focusIface+".NotificationsChanged":
			if id, _ := s.Body[0].(string); id != "" {
				select {
				case d.NotifNew <- id:
				default:
				}
			}
			kick(d.NotifsChanged)
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
		case s.Name == "org.freedesktop.DBus.Properties.PropertiesChanged" && len(s.Body) >= 2:
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

func (d *Desktop) shell(method string, args ...any) *dbus.Call {
	return d.conn.Object(focusName, focusPath).Call(focusIface+"."+method, 0, args...)
}

// D-Bus structs (a(ssss)) decode into Go structs with matching fields, not [][]string.
func (d *Desktop) Windows() ([]Window, error) {
	var raw []struct{ ID, App, Title, Icon string }
	if err := d.shell("Windows").Store(&raw); err != nil {
		return nil, err
	}
	out := make([]Window, 0, len(raw))
	for _, r := range raw {
		out = append(out, Window{ID: r.ID, App: r.App, Title: r.Title, Icon: r.Icon})
	}
	return out, nil
}

func (d *Desktop) Activate(id string) error { return d.shell("Activate", id).Err }

func (d *Desktop) Notifications() ([]Notification, error) {
	var raw []struct{ ID, App, Title, Body, Icon string }
	if err := d.shell("Notifications").Store(&raw); err != nil {
		return nil, err
	}
	out := make([]Notification, 0, len(raw))
	for _, r := range raw {
		out = append(out, Notification{ID: r.ID, App: r.App, Title: r.Title, Body: r.Body, Icon: r.Icon})
	}
	return out, nil
}

// NotificationAction: "open" or "dismiss".
func (d *Desktop) NotificationAction(id, action string) error {
	return d.shell("NotificationAction", id, action).Err
}

func (d *Desktop) Clipboard() ([]string, error) {
	var out []string
	err := d.shell("ClipboardHistory").Store(&out)
	return out, err
}

func (d *Desktop) SetClipboard(text string) error { return d.shell("SetClipboard", text).Err }

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
