package desktop

import (
	"log"
	"os"
	"strings"

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

// platform: the session bus and the MPRIS players seen on it.
type platform struct {
	conn    *dbus.Conn
	players map[string]*player // by owner
	seq     int
}

// KLID is Windows' keyboard layout of the focused window; Linux has none.
func (d *Desktop) KLID() (string, bool) { return "", false }

func Connect() (*Desktop, error) {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return nil, err
	}
	d := newDesktop()
	d.conn, d.players = conn, map[string]*player{}
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
