// Package udev installs the rule that lets the desktop user open the board's
// serial ports. Without it the daemon gets "permission denied" on /dev/ttyACM*.
// Used by `roundbar udev` / `roundbar install` (sudo or pkexec) and by the
// configurator's setup (pkexec: there is no terminal to ask in).
package udev

import (
	"os"
	"os/exec"
	"strings"

	"github.com/beedywool-vibecoded/roundbar/deploy"
)

// EtcRule is where the rule is installed.
const EtcRule = "/etc/udev/rules.d/" + deploy.UdevRuleName

// RulePaths: an installed rule in any of these (packages use /usr/lib).
var RulePaths = []string{
	EtcRule,
	"/usr/lib/udev/rules.d/" + deploy.UdevRuleName,
	"/lib/udev/rules.d/" + deploy.UdevRuleName,
}

// Script, run as root with the rule on stdin, writes it and reloads udev.
const Script = `set -e
mkdir -p /etc/udev/rules.d
cat > ` + EtcRule + `
chmod 644 ` + EtcRule + `
udevadm control --reload
udevadm trigger --subsystem-match=tty --action=change
`

// Installed reports whether a rule is in place.
func Installed() bool {
	for _, p := range RulePaths {
		if _, err := os.Stat(p); err == nil {
			return true
		}
	}
	return false
}

// Pkexec runs the script through pkexec (the desktop's password prompt), then
// restarts a running daemon so it opens the port with the new permissions.
// Replaceable in tests.
var Pkexec = func() error {
	if _, err := exec.LookPath("pkexec"); err != nil {
		return err
	}
	cmd := exec.Command("pkexec", "sh", "-c", Script)
	cmd.Stdin = strings.NewReader(deploy.UdevRule)
	if err := cmd.Run(); err != nil {
		return err
	}
	RestartDaemon()
	return nil
}

// RestartDaemon: a daemon that already failed to open the port waits for a
// hotplug event; restart it so it tries again.
func RestartDaemon() {
	_ = exec.Command("systemctl", "--user", "try-restart", "roundbar.service").Run()
}
