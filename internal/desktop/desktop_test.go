package desktop

import "testing"

// Talks to the running session (on GNOME, the extension); skipped when nothing answers.
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
	t.Logf("%+v: %d windows, %d notifications", d.Info(), len(ws), len(ns))
}
