package desktop

import "testing"

// Talks to the running GNOME Shell extension; skipped when it isn't there.
func TestShellLists(t *testing.T) {
	d, err := Connect()
	if err != nil {
		t.Skip(err)
	}
	defer d.Close()
	if _, ok := d.CurrentFocus(); !ok {
		t.Skip("extension not running")
	}
	ws, err := d.Windows()
	if err != nil {
		t.Fatal(err)
	}
	ns, err := d.Notifications()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%d windows, %d notifications", len(ws), len(ns))
}
