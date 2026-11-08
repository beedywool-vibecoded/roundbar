package daemon

import (
	"log"
	"os/exec"
	"strings"
	"syscall"
)

func spawn(name string, args ...string) {
	c := exec.Command(name, args...)
	c.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	c.Stdin, c.Stdout, c.Stderr = nil, nil, nil
	if err := c.Start(); err != nil {
		log.Printf("run %s: %v", name, err)
		return
	}
	go c.Wait() // reap
}

// runCommand runs a `command` action with sh -c, detached.
func runCommand(cmd string) { spawn("sh", "-c", cmd) }

// openTarget opens a URL or file (xdg-open) or starts a .desktop app (gtk-launch).
func openTarget(target string) {
	if strings.HasSuffix(target, ".desktop") {
		spawn("gtk-launch", strings.TrimSuffix(target, ".desktop"))
	} else {
		spawn("xdg-open", target)
	}
}

// notifyUser shows a desktop notification with a sound (timer pages).
func notifyUser(title, body string) {
	spawn("notify-send", "-a", "Roundbar", "-i", "alarm-symbolic", "-u", "critical", title, body)
	if p, err := exec.LookPath("canberra-gtk-play"); err == nil {
		spawn(p, "-i", "complete", "-d", "Roundbar timer")
	}
}
