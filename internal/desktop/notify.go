//go:build linux

package desktop

import (
	"html"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/godbus/dbus/v5"
)

// notifyMonitor mirrors desktop notifications where the shell has no API for
// them: a monitor connection sees Notify calls to the notification server
// (org.freedesktop.Notifications, whichever program owns it), their replies
// (the ids) and NotificationClosed. Content stays in this process.
type notifyMonitor struct {
	none
	ev  events
	win Shell      // finds the app's window to "open" a notification
	bus *dbus.Conn // regular connection, for CloseNotification
	mon *dbus.Conn // monitor connection: receives only, can't send

	mu      sync.Mutex
	list    []notifEntry // oldest first
	pending map[pendKey]notifEntry

	done      chan struct{}
	closeOnce sync.Once
}

type notifEntry struct {
	id      uint32
	n       Notification
	appName string
	desktop string // desktop-entry hint
}

type pendKey struct {
	peer   string
	serial uint32
}

const (
	notifName  = "org.freedesktop.Notifications"
	notifPath  = "/org/freedesktop/Notifications"
	maxNotifs  = 20
	maxPending = 64
)

func newNotifyMonitor(bus *dbus.Conn, ev events, win Shell) (*notifyMonitor, error) {
	return startNotifyMonitor(bus, ev, win, notifName)
}

func startNotifyMonitor(bus *dbus.Conn, ev events, win Shell, server string) (*notifyMonitor, error) {
	mon, err := dbus.ConnectSessionBus()
	if err != nil {
		return nil, err
	}
	m := &notifyMonitor{ev: ev, win: win, bus: bus, mon: mon, pending: map[pendKey]notifEntry{}, done: make(chan struct{})}
	ch := make(chan *dbus.Message, 64)
	mon.Eavesdrop(ch) // from here on every message lands in ch, replies included
	rules := []string{
		"type='method_call',interface='org.freedesktop.Notifications',member='Notify'",
		"type='method_return',sender='" + server + "'",
		"type='signal',interface='org.freedesktop.Notifications',member='NotificationClosed'",
	}
	call := mon.BusObject().Call("org.freedesktop.DBus.Monitoring.BecomeMonitor", dbus.FlagNoReplyExpected, rules, uint32(0))
	if call.Err != nil {
		mon.Close()
		return nil, call.Err
	}
	go m.loop(ch)
	return m, nil
}

func (m *notifyMonitor) Caps() Caps { return Caps{Notifications: true} }

// Close detaches the eavesdrop channel before closing the connection: godbus
// closes that channel in Close while its reader can still deliver one last
// message to it (send on a closed channel). With the channel detached under
// godbus's lock, nothing writes to it any more; the loop ends on done.
func (m *notifyMonitor) Close() {
	m.closeOnce.Do(func() {
		m.mon.Eavesdrop(nil)
		close(m.done)
		m.mon.Close()
	})
}

func (m *notifyMonitor) loop(ch chan *dbus.Message) {
	for {
		var msg *dbus.Message
		select {
		case <-m.done:
			return
		case got, ok := <-ch:
			if !ok { // the bus went away (godbus closed it)
				return
			}
			msg = got
		}
		m.handle(msg)
	}
}

func (m *notifyMonitor) handle(msg *dbus.Message) {
	str := func(f dbus.HeaderField) string { s, _ := msg.Headers[f].Value().(string); return s }
	switch msg.Type {
	case dbus.TypeMethodCall:
		if str(dbus.FieldMember) != "Notify" || len(msg.Body) < 7 {
			return
		}
		e, ok := parseNotify(msg.Body)
		if !ok {
			return
		}
		m.mu.Lock()
		if len(m.pending) >= maxPending { // replies we never saw
			clear(m.pending)
		}
		m.pending[pendKey{str(dbus.FieldSender), msg.Serial()}] = e
		m.mu.Unlock()
	case dbus.TypeMethodReply:
		serial, _ := msg.Headers[dbus.FieldReplySerial].Value().(uint32)
		k := pendKey{str(dbus.FieldDestination), serial}
		m.mu.Lock()
		e, ok := m.pending[k]
		delete(m.pending, k)
		m.mu.Unlock()
		if !ok || len(msg.Body) < 1 {
			return
		}
		id, _ := msg.Body[0].(uint32)
		if id != 0 {
			m.add(id, e)
		}
	case dbus.TypeSignal:
		if str(dbus.FieldMember) == "NotificationClosed" && len(msg.Body) >= 1 {
			if id, ok := msg.Body[0].(uint32); ok && m.remove(id) {
				m.ev.notif("")
			}
		}
	}
}

var tags = regexp.MustCompile(`<[^>]*>`)

// parseNotify reads Notify(app_name, replaces_id, app_icon, summary, body, actions, hints, timeout).
func parseNotify(body []any) (notifEntry, bool) {
	appName, ok1 := body[0].(string)
	icon, _ := body[2].(string)
	summary, ok2 := body[3].(string)
	text, _ := body[4].(string)
	hints, _ := body[6].(map[string]dbus.Variant)
	if !ok1 || !ok2 {
		return notifEntry{}, false
	}
	e := notifEntry{appName: appName}
	if v, ok := hints["desktop-entry"]; ok {
		e.desktop, _ = v.Value().(string)
	}
	app := appName
	if a, ok := lookupApp(e.desktop, appName); ok {
		if a.name != "" {
			app = a.name
		}
		if icon == "" {
			icon = a.icon
		}
	}
	if app == "" {
		app = e.desktop
	}
	text = html.UnescapeString(tags.ReplaceAllString(text, "")) // body may carry simple markup
	e.n = Notification{App: app, Title: summary, Body: strings.TrimSpace(text), Icon: icon}
	return e, true
}

func (m *notifyMonitor) add(id uint32, e notifEntry) {
	e.id = id
	e.n.ID = strconv.FormatUint(uint64(id), 10)
	m.mu.Lock()
	replaced := false
	for i := range m.list {
		if m.list[i].id == id { // replaces_id: update in place
			m.list[i] = e
			replaced = true
		}
	}
	if !replaced {
		m.list = append(m.list, e)
		if len(m.list) > maxNotifs {
			m.list = m.list[len(m.list)-maxNotifs:]
		}
	}
	m.mu.Unlock()
	if replaced {
		m.ev.notif("")
	} else {
		m.ev.notif(e.n.ID)
	}
}

func (m *notifyMonitor) remove(id uint32) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.list {
		if m.list[i].id == id {
			m.list = append(m.list[:i], m.list[i+1:]...)
			return true
		}
	}
	return false
}

func (m *notifyMonitor) Notifications() ([]Notification, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Notification, 0, len(m.list))
	for i := len(m.list) - 1; i >= 0; i-- {
		out = append(out, m.list[i].n)
	}
	return out, nil
}

// NotificationAction: "open" brings the sender's window forward, both close it.
func (m *notifyMonitor) NotificationAction(id, action string) error {
	n, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		return err
	}
	if action == "open" {
		m.mu.Lock()
		var e notifEntry
		for _, x := range m.list {
			if x.id == uint32(n) {
				e = x
			}
		}
		m.mu.Unlock()
		if ws, err := m.win.Windows(); err == nil {
			if w, ok := windowOf(ws, e.desktop, e.appName); ok {
				_ = m.win.Activate(w.ID)
			}
		}
	}
	if m.remove(uint32(n)) {
		m.ev.notif("")
	}
	return m.bus.Object(notifName, notifPath).Call(notifName+".CloseNotification", 0, uint32(n)).Err
}

// windowOf finds the most recent window of the app behind a notification.
func windowOf(ws []Window, desktopEntry, appName string) (Window, bool) {
	var keys []string
	for _, k := range []string{desktopEntry, appName} {
		if k != "" {
			keys = append(keys, strings.ToLower(k))
		}
	}
	if a, ok := lookupApp(desktopEntry, appName); ok {
		keys = append(keys, strings.ToLower(a.id))
	}
	for _, w := range ws {
		app := strings.ToLower(w.App)
		short := app[strings.LastIndexByte(app, '.')+1:]
		for _, k := range keys {
			if app == k || short == k {
				return w, true
			}
		}
	}
	return Window{}, false
}
