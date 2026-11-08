// Package deploy embeds the system files `roundbar install` puts in place and
// the packages (packaging/) ship.
package deploy

import _ "embed"

// UdevRule gives the logged-in desktop user access to Espressif serial ports.
//
//go:embed 70-roundbar.rules
var UdevRule string

// UdevRuleName is the file name under /etc/udev/rules.d or /usr/lib/udev/rules.d.
const UdevRuleName = "70-roundbar.rules"
