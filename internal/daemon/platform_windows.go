package daemon

import (
	"log"
	"os/exec"
	"syscall"
	_ "time/tzdata" // world clocks: Windows has no IANA zone database for time.LoadLocation

	"github.com/beedywool-vibecoded/roundbar/internal/win"
)

// Notify shows a notification from the tray icon (set by the tray app). The
// timer page uses it; without a tray nothing is shown.
var Notify func(title, body string)

const createNoWindow = 0x08000000

// runCommand runs a `command` action through cmd.exe, without a console window.
func runCommand(cmd string) {
	c := exec.Command("cmd.exe")
	c.SysProcAttr = &syscall.SysProcAttr{
		CmdLine:       `cmd.exe /d /s /c "` + cmd + `"`, // cmd.exe's own quoting: everything between the outer quotes
		HideWindow:    true,
		CreationFlags: createNoWindow | syscall.CREATE_NEW_PROCESS_GROUP,
	}
	if err := c.Start(); err != nil {
		log.Printf("run %s: %v", cmd, err)
		return
	}
	go c.Wait()
}

// openTarget hands a URL, file, folder or program to the shell (ShellExecute "open").
func openTarget(target string) {
	go func() {
		if err := win.ShellOpen(target, ""); err != nil {
			log.Printf("open %s: %v", target, err)
		}
	}()
}

func notifyUser(title, body string) {
	if Notify != nil {
		Notify(title, body)
	}
}
