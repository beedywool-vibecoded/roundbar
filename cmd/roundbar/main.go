// roundbar: daemon, configurator and helper commands for the round macro controller.
package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	gnomeext "github.com/beedywool-vibecoded/roundbar/gnome-extension"
	"github.com/beedywool-vibecoded/roundbar/internal/configui"
	"github.com/beedywool-vibecoded/roundbar/internal/daemon"
	"github.com/beedywool-vibecoded/roundbar/internal/ipc"
)

const usage = `usage: roundbar <command>

  daemon       run the bridge between the device and the desktop (systemd user service)
  config       open the configurator in the browser (--no-open to only print the URL)
  status       print daemon and device status
  bootloader   reboot the device into USB download mode, for flashing
  install      install the systemd user service, GNOME extension and app launcher
`

func main() {
	log.SetFlags(0)
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "daemon":
		err = daemon.New().Run()
	case "config":
		open := !(len(os.Args) > 2 && os.Args[2] == "--no-open")
		err = configui.Run(open)
	case "status":
		var st json.RawMessage
		if err = ipc.Call(ipc.Request{Op: "status"}, &st); err == nil {
			var pretty map[string]any
			_ = json.Unmarshal(st, &pretty)
			b, _ := json.MarshalIndent(pretty, "", "  ")
			fmt.Println(string(b))
		} else {
			err = fmt.Errorf("daemon not reachable (%v); start it with: systemctl --user start roundbar", err)
		}
	case "bootloader":
		err = ipc.Call(ipc.Request{Op: "bootloader"}, nil)
	case "install":
		err = install()
	default:
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	if err != nil {
		log.Fatal(err)
	}
}

const unit = `[Unit]
Description=roundbar device bridge
PartOf=graphical-session.target
After=graphical-session.target

[Service]
ExecStart=%s daemon
Restart=on-failure
RestartSec=3
Nice=5
# It idles almost all the time; keep it off the CPU's fast path.
CPUSchedulingPolicy=batch
MemoryHigh=64M

[Install]
WantedBy=graphical-session.target
`

const desktopEntry = `[Desktop Entry]
Type=Application
Name=roundbar
Comment=Configure the round macro controller
Exec=%s config
Icon=input-dialpad
Terminal=false
Categories=Settings;HardwareSettings;
`

func install() error {
	self, err := os.Executable()
	if err != nil {
		return err
	}
	self, _ = filepath.EvalSymlinks(self)
	home, _ := os.UserHomeDir()

	write := func(path, content string) error {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		fmt.Println("wrote", path)
		return os.WriteFile(path, []byte(content), 0o644)
	}
	if err := write(filepath.Join(home, ".config/systemd/user/roundbar.service"), fmt.Sprintf(unit, self)); err != nil {
		return err
	}
	if err := write(filepath.Join(home, ".local/share/applications/roundbar.desktop"), fmt.Sprintf(desktopEntry, self)); err != nil {
		return err
	}

	extDir := filepath.Join(home, ".local/share/gnome-shell/extensions", gnomeext.UUID)
	err = fs.WalkDir(gnomeext.Files, gnomeext.UUID, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := gnomeext.Files.ReadFile(p)
		if err != nil {
			return err
		}
		return write(filepath.Join(extDir, strings.TrimPrefix(p, gnomeext.UUID+"/")), string(b))
	})
	if err != nil {
		return err
	}

	run := func(name string, args ...string) {
		out, err := exec.Command(name, args...).CombinedOutput()
		if err != nil {
			fmt.Printf("  %s %s: %v %s\n", name, strings.Join(args, " "), err, strings.TrimSpace(string(out)))
		}
	}
	run("systemctl", "--user", "daemon-reload")
	run("systemctl", "--user", "enable", "--now", "roundbar.service")
	run("systemctl", "--user", "restart", "roundbar.service")
	run("gnome-extensions", "enable", gnomeext.UUID)
	fmt.Println(`
done. On Wayland GNOME only loads a newly installed extension after you log out and back in;
until then the daemon runs with the default profile.`)
	return nil
}
