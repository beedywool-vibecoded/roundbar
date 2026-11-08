package main

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	gnomeext "github.com/beedywool-vibecoded/roundbar/gnome-extension"
	"github.com/beedywool-vibecoded/roundbar/internal/daemon"
	"github.com/beedywool-vibecoded/roundbar/internal/desktop"
)

const usage = `usage: roundbar <command>

  daemon       run the bridge between the device and the desktop (systemd user service)
  config       open the configurator in the browser (--no-open to only print the URL)
  status       print daemon and device status
  bootloader   reboot the device into USB download mode, for flashing
  screenshot   save what the device shows as a PNG (roundbar screenshot out.png)
  install      install the systemd user service, app launcher (and on GNOME its extension)
`

const startHint = "systemctl --user start roundbar"

var defaultCommand []string // a command is required

func setup(string) {}

func runDaemon() error { return daemon.New().Run() }

func platformCommand(string) (bool, error) { return false, nil }

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

	run := func(name string, args ...string) {
		out, err := exec.Command(name, args...).CombinedOutput()
		if err != nil {
			fmt.Printf("  %s %s: %v %s\n", name, strings.Join(args, " "), err, strings.TrimSpace(string(out)))
		}
	}
	backend := desktop.Detect()
	if backend != desktop.BackendGnome {
		run("systemctl", "--user", "daemon-reload")
		run("systemctl", "--user", "enable", "--now", "roundbar.service")
		run("systemctl", "--user", "restart", "roundbar.service")
		fmt.Printf("\ndone. Desktop backend: %s (no extension needed).\n", backend)
		return nil
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
	run("systemctl", "--user", "daemon-reload")
	run("systemctl", "--user", "enable", "--now", "roundbar.service")
	run("systemctl", "--user", "restart", "roundbar.service")
	// GNOME can't enable an extension it hasn't scanned yet (new installs on
	// Wayland need a re-login), so put it in the enabled list directly.
	if err := exec.Command("gnome-extensions", "enable", gnomeext.UUID).Run(); err != nil {
		out, _ := exec.Command("gsettings", "get", "org.gnome.shell", "enabled-extensions").Output()
		list := strings.TrimSpace(string(out))
		if !strings.Contains(list, gnomeext.UUID) {
			switch {
			case list == "@as []" || list == "[]" || list == "":
				list = "['" + gnomeext.UUID + "']"
			default:
				list = strings.TrimSuffix(list, "]") + ", '" + gnomeext.UUID + "']"
			}
			run("gsettings", "set", "org.gnome.shell", "enabled-extensions", list)
		}
	}
	fmt.Println(`
done. On Wayland GNOME only loads a newly installed extension after you log out and back in;
until then the daemon runs with the default profile.`)
	return nil
}
