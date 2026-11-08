package desktop

import "github.com/godbus/dbus/v5"

// gnome talks to our GNOME Shell extension (gnome-extension/) on the session
// bus. It covers everything; its signals carry no private data.
type gnome struct {
	conn *dbus.Conn
	ev   events
	sig  chan *dbus.Signal
}

const (
	focusName  = "dev.roundbar.Focus"
	focusPath  = "/dev/roundbar/Focus"
	focusIface = "dev.roundbar.Focus"
)

func newGnome(conn *dbus.Conn, ev events) (*gnome, error) {
	if err := conn.AddMatchSignal(dbus.WithMatchInterface(focusIface), dbus.WithMatchObjectPath(focusPath)); err != nil {
		return nil, err
	}
	g := &gnome{conn: conn, ev: ev, sig: make(chan *dbus.Signal, 16)}
	conn.Signal(g.sig)
	go g.loop()
	return g, nil
}

func (g *gnome) loop() {
	for s := range g.sig {
		if s.Path != focusPath {
			continue
		}
		switch s.Name {
		case focusIface + ".Changed":
			if len(s.Body) >= 3 {
				f := Focus{}
				f.ID, _ = s.Body[0].(string)
				f.Name, _ = s.Body[1].(string)
				f.Class, _ = s.Body[2].(string)
				g.ev.focus(f)
			}
		case focusIface + ".WindowsChanged":
			g.ev.windows()
		case focusIface + ".ClipboardChanged":
			g.ev.clips()
		case focusIface + ".NotificationsChanged":
			id := ""
			if len(s.Body) > 0 {
				id, _ = s.Body[0].(string)
			}
			g.ev.notif(id)
		}
	}
}

func (g *gnome) Caps() Caps { return Caps{true, true, true, true} }

func (g *gnome) Close() { g.conn.RemoveSignal(g.sig) }

func (g *gnome) call(method string, args ...any) *dbus.Call {
	return g.conn.Object(focusName, focusPath).Call(focusIface+"."+method, 0, args...)
}

func (g *gnome) Focus() (Focus, error) {
	var f Focus
	err := g.call("Get").Store(&f.ID, &f.Name, &f.Class)
	return f, err
}

// D-Bus structs (a(ssss)) decode into Go structs with matching fields, not [][]string.
func (g *gnome) Windows() ([]Window, error) {
	var raw []struct{ ID, App, Title, Icon string }
	if err := g.call("Windows").Store(&raw); err != nil {
		return nil, err
	}
	out := make([]Window, 0, len(raw))
	for _, r := range raw {
		out = append(out, Window{ID: r.ID, App: r.App, Title: r.Title, Icon: r.Icon})
	}
	return out, nil
}

func (g *gnome) Activate(id string) error { return g.call("Activate", id).Err }

func (g *gnome) Notifications() ([]Notification, error) {
	var raw []struct{ ID, App, Title, Body, Icon string }
	if err := g.call("Notifications").Store(&raw); err != nil {
		return nil, err
	}
	out := make([]Notification, 0, len(raw))
	for _, r := range raw {
		out = append(out, Notification{ID: r.ID, App: r.App, Title: r.Title, Body: r.Body, Icon: r.Icon})
	}
	return out, nil
}

func (g *gnome) NotificationAction(id, action string) error {
	return g.call("NotificationAction", id, action).Err
}

func (g *gnome) Clipboard() ([]string, error) {
	var out []string
	err := g.call("ClipboardHistory").Store(&out)
	return out, err
}

func (g *gnome) SetClipboard(text string) error { return g.call("SetClipboard", text).Err }
