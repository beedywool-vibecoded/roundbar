//go:build linux

package desktop

import (
	"testing"

	"github.com/godbus/dbus/v5"
)

func TestParseNotify(t *testing.T) {
	testApps(t, map[string]string{"org.mozilla.firefox": "[Desktop Entry]\nName=Firefox\nIcon=firefox\n"})
	hints := map[string]dbus.Variant{"desktop-entry": dbus.MakeVariant("org.mozilla.firefox")}
	e, ok := parseNotify([]any{"Firefox", uint32(0), "", "Download done", "<b>file.zip</b> &amp; more", []string{}, hints, int32(-1)})
	if !ok {
		t.Fatal("not parsed")
	}
	want := Notification{App: "Firefox", Title: "Download done", Body: "file.zip & more", Icon: "firefox"}
	if e.n != want || e.desktop != "org.mozilla.firefox" {
		t.Fatalf("%+v", e)
	}
	if _, ok := parseNotify([]any{1, 2, 3, 4, 5, 6, 7, 8}); ok {
		t.Fatal("garbage parsed")
	}
}

func TestWindowOf(t *testing.T) {
	testApps(t, nil)
	ws := []Window{{ID: "1", App: "kitty"}, {ID: "2", App: "org.mozilla.firefox"}, {ID: "3", App: "firefox"}}
	if w, ok := windowOf(ws, "", "Firefox"); !ok || w.ID != "2" {
		t.Fatal(w, ok)
	}
	if _, ok := windowOf(ws, "", "Signal"); ok {
		t.Fatal("matched nothing")
	}
}

// fakeServer plays the notification server: Notify returns increasing ids,
// CloseNotification emits NotificationClosed.
type fakeServer struct {
	conn   *dbus.Conn
	next   uint32
	closed chan uint32
}

func (s *fakeServer) Notify(app string, replaces uint32, icon, summary, body string, actions []string, hints map[string]dbus.Variant, timeout int32) (uint32, *dbus.Error) {
	if replaces != 0 {
		return replaces, nil
	}
	s.next++
	return s.next, nil
}

func (s *fakeServer) CloseNotification(id uint32) *dbus.Error {
	s.closed <- id
	s.conn.Emit(notifPath, notifName+".NotificationClosed", id, uint32(3))
	return nil
}

// A throwaway bus with a fake notification server: the monitor sees Notify,
// matches the reply id and follows NotificationClosed.
func TestNotifyMonitor(t *testing.T) {
	privateBus(t)
	testApps(t, nil)
	srvConn, err := dbus.ConnectSessionBus()
	if err != nil {
		t.Fatal(err)
	}
	defer srvConn.Close()
	srv := &fakeServer{conn: srvConn, closed: make(chan uint32, 4)}
	if err := srvConn.Export(srv, notifPath, notifName); err != nil {
		t.Fatal(err)
	}
	if r, err := srvConn.RequestName(notifName, 0); err != nil || r != dbus.RequestNameReplyPrimaryOwner {
		t.Fatal(r, err)
	}

	bus, err := dbus.ConnectSessionBus()
	if err != nil {
		t.Fatal(err)
	}
	defer bus.Close()
	rec := newRecorder()
	win := &fakeWins{}
	m, err := newNotifyMonitor(bus, rec, win)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()

	app, err := dbus.ConnectSessionBus()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	notify := func(replaces uint32, summary string) uint32 {
		var id uint32
		hints := map[string]dbus.Variant{"desktop-entry": dbus.MakeVariant("kitty")}
		err := app.Object(notifName, notifPath).Call(notifName+".Notify", 0, "kitty", replaces, "", summary, "body", []string{}, hints, int32(-1)).Store(&id)
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	var id uint32
	for i := 0; i < 50 && id == 0; i++ { // the monitor may not be set up yet: retry until it sees one
		n := notify(0, "hello")
		select {
		case got := <-rec.notifC:
			if got != "" {
				id = n
			}
		case <-waitShort():
		}
	}
	if id == 0 {
		t.Fatal("monitor saw no notification")
	}
	list, _ := m.Notifications()
	if len(list) == 0 || list[0].Title != "hello" || list[0].App != "kitty" {
		t.Fatalf("%+v", list)
	}

	notify(id, "updated") // replaces_id: same entry, no new toast
	if got := wait(t, rec.notifC, "replace"); got != "" {
		t.Fatalf("replacement announced as new: %q", got)
	}
	list, _ = m.Notifications()
	if list[0].Title != "updated" {
		t.Fatalf("%+v", list)
	}

	if err := m.NotificationAction(list[0].ID, "open"); err != nil {
		t.Fatal(err)
	}
	if win.activated != "w1" {
		t.Fatalf("activated %q", win.activated)
	}
	if got := wait(t, srv.closed, "close"); got != id {
		t.Fatal(got)
	}
	for _, n := range must(m.Notifications()) {
		if n.ID == list[0].ID {
			t.Fatal("still listed after open")
		}
	}
}

type fakeWins struct {
	none
	activated string
}

func (f *fakeWins) Windows() ([]Window, error) {
	return []Window{{ID: "w0", App: "firefox"}, {ID: "w1", App: "kitty"}}, nil
}
func (f *fakeWins) Activate(id string) error { f.activated = id; return nil }

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}
