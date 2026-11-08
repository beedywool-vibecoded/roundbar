package daemon

import (
	"log"

	"github.com/beedywool-vibecoded/roundbar/internal/keymap"
)

// detectLayout reads the PC's active keyboard layout and tells the device, which
// uses it when device.keyboardLayout is "auto". Runs on connect (force) and when the
// desktop signals a layout change; nothing polls.
func (d *Daemon) detectLayout(force bool) {
	d.layoutMu.Lock()
	defer d.layoutMu.Unlock()
	env := keymap.SystemEnv()
	if d.desk != nil {
		env.KDEIndex = d.desk.KDELayoutIndex
	}
	det := keymap.Detect(env)
	d.mu.Lock()
	changed := det != d.layout
	d.layout = det
	d.mu.Unlock()
	if changed {
		log.Printf("keyboard layout: %q (%s)", det.ID, det.Source)
		d.notify()
	}
	if det.ID != "" && (changed || force) {
		d.send(map[string]any{"t": "layout", "id": det.ID})
	}
}
