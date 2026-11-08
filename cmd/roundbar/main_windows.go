package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"unsafe"

	"github.com/beedywool-vibecoded/roundbar/internal/daemon"
	"github.com/beedywool-vibecoded/roundbar/internal/win"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const usage = `usage: roundbar [command]

  tray         the daemon with a notification-area icon (the default; started at login)
  daemon       run the bridge between the device and the desktop, without the icon
  config       open the configurator in the browser (--no-open to only print the URL)
  status       print daemon and device status
  bootloader   reboot the device into USB download mode, for flashing
  screenshot   save what the device shows as a PNG (roundbar screenshot out.png)
  install      start at login and add a Start menu shortcut
  uninstall    undo install
`

const startHint = "the Roundbar app (Start menu)"

// Double-clicking the exe (or the Start menu entry) starts the tray app.
var defaultCommand = []string{"tray"}

var (
	procAttachConsole = win.Kernel32("AttachConsole")
	procCreateMutexW  = win.Kernel32("CreateMutexW")
)

// setup gives the GUI-subsystem exe somewhere to write: the terminal it was
// started from for the command-line commands, a log file for the background ones.
func setup(cmd string) {
	switch cmd {
	case "tray", "config":
		if f := openLog(); f != nil {
			log.SetOutput(f)
			log.SetFlags(log.LstdFlags)
			_ = debug.SetCrashOutput(f, debug.CrashOptions{}) // a crash leaves its trace there too
		}
		return
	}
	const attachParent = ^uintptr(0) // ATTACH_PARENT_PROCESS
	if r, _, _ := procAttachConsole.Call(attachParent); r == 0 {
		return
	}
	if out, err := os.OpenFile("CONOUT$", os.O_WRONLY, 0); err == nil {
		os.Stdout, os.Stderr = out, out
		log.SetOutput(out)
		fmt.Fprintln(out) // the prompt came back before us: start on a fresh line
	}
}

// openLog appends to %LOCALAPPDATA%\roundbar\roundbar.log, starting over past 1 MB.
func openLog() *os.File {
	dir := filepath.Join(os.Getenv("LOCALAPPDATA"), "roundbar")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil
	}
	path := filepath.Join(dir, "roundbar.log")
	if st, err := os.Stat(path); err == nil && st.Size() > 1<<20 {
		_ = os.Rename(path, path+".old")
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil
	}
	return f
}

var errRunning = errors.New("roundbar is already running")

// singleInstance holds a per-session named mutex for the life of the process.
func singleInstance() error {
	name, _ := windows.UTF16PtrFromString(`Local\roundbar`)
	h, _, err := procCreateMutexW.Call(0, 0, uintptr(unsafe.Pointer(name)))
	if h == 0 {
		return err
	}
	if errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
		return errRunning
	}
	return nil // the handle stays open until we exit
}

func runDaemon() error {
	if err := singleInstance(); err != nil {
		return err
	}
	return daemon.New().Run()
}

func platformCommand(cmd string) (bool, error) {
	switch cmd {
	case "tray":
		if err := singleInstance(); errors.Is(err, errRunning) {
			return true, openConfigurator() // started again: the user wants to see it
		} else if err != nil {
			return true, err
		}
		return true, runTray()
	case "uninstall":
		return true, uninstall()
	}
	return false, nil
}

// openConfigurator starts `roundbar config` (it opens the browser and exits
// when the last tab closes).
func openConfigurator() error {
	self, err := os.Executable()
	if err != nil {
		return err
	}
	c := exec.Command(self, "config")
	if err := c.Start(); err != nil {
		return err
	}
	go c.Wait()
	return nil
}

const runKey = `Software\Microsoft\Windows\CurrentVersion\Run`

func selfPath() string {
	p, _ := os.Executable()
	if r, err := filepath.EvalSymlinks(p); err == nil {
		p = r
	}
	return p
}

func autostartCommand() string { return `"` + selfPath() + `" tray` }

// autostart reports whether the Run key starts this exe at login.
func autostart() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	v, _, err := k.GetStringValue("Roundbar")
	return err == nil && v != ""
}

func setAutostart(on bool) error {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, runKey, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	if on {
		return k.SetStringValue("Roundbar", autostartCommand())
	}
	if err := k.DeleteValue("Roundbar"); err != nil && !errors.Is(err, registry.ErrNotExist) {
		return err
	}
	return nil
}

func shortcutPath() string {
	return filepath.Join(os.Getenv("APPDATA"), `Microsoft\Windows\Start Menu\Programs`, "Roundbar.lnk")
}

func install() error {
	if err := setAutostart(true); err != nil {
		return err
	}
	fmt.Println("start at login:", autostartCommand())
	if err := win.CreateShortcut(shortcutPath(), selfPath(), "", "Roundbar: the round macro controller"); err != nil {
		return err
	}
	fmt.Println("wrote", shortcutPath())
	fmt.Println("\ndone. Start Roundbar from the Start menu (it then starts at every login).")
	return nil
}

func uninstall() error {
	if err := setAutostart(false); err != nil {
		return err
	}
	if err := os.Remove(shortcutPath()); err != nil && !os.IsNotExist(err) {
		return err
	}
	fmt.Println("removed the login entry and the Start menu shortcut")
	return nil
}
