package main

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/beedywool-vibecoded/roundbar/deploy"
	gnomeext "github.com/beedywool-vibecoded/roundbar/gnome-extension"
)

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

// Where packages put their copies; `roundbar install` doesn't shadow them.
const (
	systemUnit      = "/usr/lib/systemd/user/roundbar.service"
	systemDesktop   = "/usr/share/applications/roundbar.desktop"
	systemExtension = "/usr/share/gnome-shell/extensions/" + gnomeext.UUID
	etcUdevRule     = "/etc/udev/rules.d/" + deploy.UdevRuleName
)

var udevRulePaths = []string{
	etcUdevRule,
	"/usr/lib/udev/rules.d/" + deploy.UdevRuleName,
	"/lib/udev/rules.d/" + deploy.UdevRuleName,
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func udevRuleInstalled() bool {
	for _, p := range udevRulePaths {
		if exists(p) {
			return true
		}
	}
	return false
}

// ask asks a yes/no question on the terminal (even when stdin is a pipe, as
// in `curl … | sh`). ok is false when there is no terminal to ask on.
func ask(question string, def bool) (yes, ok bool) {
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return false, false
	}
	defer tty.Close()
	hint := " [Y/n] "
	if !def {
		hint = " [y/N] "
	}
	fmt.Fprint(tty, question+hint)
	line, err := bufio.NewReader(tty).ReadString('\n')
	if err != nil {
		return false, false
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "":
		return def, true
	case "y", "yes":
		return true, true
	default:
		return false, true
	}
}

// asRoot runs a shell script as root with stdin as its input: sudo when
// there is a terminal for the password, pkexec (graphical prompt) otherwise.
func asRoot(script, stdin string) error {
	var cmd *exec.Cmd
	_, sudoErr := exec.LookPath("sudo")
	_, pkexecErr := exec.LookPath("pkexec")
	_, ttyErr := os.Stat("/dev/tty")
	switch {
	case sudoErr == nil && (ttyErr == nil || pkexecErr != nil):
		cmd = exec.Command("sudo", "sh", "-c", script)
	case pkexecErr == nil:
		cmd = exec.Command("pkexec", "sh", "-c", script)
	default:
		return errors.New("neither sudo nor pkexec is available")
	}
	cmd.Stdin = strings.NewReader(stdin)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	return cmd.Run()
}

const udevInstallScript = `set -e
mkdir -p /etc/udev/rules.d
cat > ` + etcUdevRule + `
chmod 644 ` + etcUdevRule + `
udevadm control --reload
udevadm trigger --subsystem-match=tty --action=change
`

// installUdev installs the udev rule that lets the desktop user open the board's
// serial ports. Without it the daemon gets "permission denied" on /dev/ttyACM*.
func installUdev(prompt bool) error {
	if udevRuleInstalled() {
		fmt.Println("udev rule already installed")
		return nil
	}
	fmt.Print(`
roundbar talks to the board over a USB serial port (/dev/ttyACM*). Linux only
lets root open it until a udev rule grants access to the person logged in at
the desktop. Installing that rule (` + etcUdevRule + `) needs your password once.
`)
	if prompt {
		yes, ok := ask("Install the udev rule now?", true)
		if !ok || !yes {
			fmt.Println("skipped. To do it later, run: roundbar udev")
			return nil
		}
	}
	if err := asRoot(udevInstallScript, deploy.UdevRule); err != nil {
		return fmt.Errorf("installing the udev rule failed (%v); run `roundbar udev` to try again", err)
	}
	fmt.Println("installed", etcUdevRule)
	// A daemon that already failed to open the port waits for a hotplug event;
	// restart it so it tries again with the new permissions.
	_ = exec.Command("systemctl", "--user", "try-restart", "roundbar.service").Run()
	return nil
}

func install(args []string) error {
	noUdev := false
	for _, a := range args {
		switch a {
		case "--no-udev":
			noUdev = true
		default:
			return fmt.Errorf("unknown option %s", a)
		}
	}
	self, err := os.Executable()
	if err != nil {
		return err
	}
	self, _ = filepath.EvalSymlinks(self)
	home, _ := os.UserHomeDir()

	if !noUdev {
		if err := installUdev(true); err != nil {
			fmt.Println(err)
		}
	}

	write := func(path, content string) error {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		fmt.Println("wrote", path)
		return os.WriteFile(path, []byte(content), 0o644)
	}
	if exists(systemUnit) {
		fmt.Println("using the packaged service", systemUnit)
	} else if err := write(filepath.Join(home, ".config/systemd/user/roundbar.service"), fmt.Sprintf(unit, self)); err != nil {
		return err
	}
	if !exists(systemDesktop) {
		if err := write(filepath.Join(home, ".local/share/applications/roundbar.desktop"), fmt.Sprintf(desktopEntry, self)); err != nil {
			return err
		}
	}

	if !exists(systemExtension) {
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
	}

	run("systemctl", "--user", "daemon-reload")
	run("systemctl", "--user", "enable", "--now", "roundbar.service")
	run("systemctl", "--user", "restart", "roundbar.service")
	enableExtension()
	fmt.Println(`
done. Open the configurator from your app launcher ("roundbar") or with: roundbar config
On Wayland GNOME only loads a newly installed extension after you log out and back in;
until then the daemon runs with the default profile.`)
	return nil
}

func run(name string, args ...string) {
	out, err := exec.Command(name, args...).CombinedOutput()
	if err != nil {
		fmt.Printf("  %s %s: %v %s\n", name, strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
}

// enableExtension: GNOME can't enable an extension it hasn't scanned yet (new
// installs on Wayland need a re-login), so put it in the enabled list directly.
func enableExtension() {
	if exec.Command("gnome-extensions", "enable", gnomeext.UUID).Run() == nil {
		return
	}
	out, err := exec.Command("gsettings", "get", "org.gnome.shell", "enabled-extensions").Output()
	if err != nil {
		return // not GNOME
	}
	list := strings.TrimSpace(string(out))
	if strings.Contains(list, gnomeext.UUID) {
		return
	}
	if list == "@as []" || list == "[]" || list == "" {
		list = "['" + gnomeext.UUID + "']"
	} else {
		list = strings.TrimSuffix(list, "]") + ", '" + gnomeext.UUID + "']"
	}
	run("gsettings", "set", "org.gnome.shell", "enabled-extensions", list)
}

// disableExtension removes the extension from GNOME's enabled list.
func disableExtension() {
	if exec.Command("gnome-extensions", "disable", gnomeext.UUID).Run() == nil {
		return
	}
	out, err := exec.Command("gsettings", "get", "org.gnome.shell", "enabled-extensions").Output()
	if err != nil {
		return
	}
	list := strings.TrimSpace(string(out))
	if !strings.Contains(list, gnomeext.UUID) {
		return
	}
	list = strings.NewReplacer(", '"+gnomeext.UUID+"'", "", "'"+gnomeext.UUID+"', ", "", "'"+gnomeext.UUID+"'", "").Replace(list)
	run("gsettings", "set", "org.gnome.shell", "enabled-extensions", list)
}

// uninstall undoes `roundbar install`. Settings in ~/.config/roundbar stay.
func uninstall(args []string) error {
	keepBinary := false
	for _, a := range args {
		switch a {
		case "--keep-binary":
			keepBinary = true
		default:
			return fmt.Errorf("unknown option %s", a)
		}
	}
	home, _ := os.UserHomeDir()
	run("systemctl", "--user", "disable", "--now", "roundbar.service")
	disableExtension()
	remove := func(p string) {
		if err := os.RemoveAll(p); err == nil {
			fmt.Println("removed", p)
		}
	}
	for _, p := range []string{
		filepath.Join(home, ".config/systemd/user/roundbar.service"),
		filepath.Join(home, ".local/share/applications/roundbar.desktop"),
		filepath.Join(home, ".local/share/gnome-shell/extensions", gnomeext.UUID),
	} {
		if exists(p) {
			remove(p)
		}
	}
	run("systemctl", "--user", "daemon-reload")

	if exists(etcUdevRule) {
		if yes, ok := ask("Remove the udev rule "+etcUdevRule+" (needs your password)?", true); ok && yes {
			if err := asRoot("rm -f "+etcUdevRule+" && udevadm control --reload", ""); err != nil {
				fmt.Println("couldn't remove it:", err)
			} else {
				fmt.Println("removed", etcUdevRule)
			}
		} else {
			fmt.Println("kept", etcUdevRule, "(remove it with: sudo rm "+etcUdevRule+")")
		}
	}

	if !keepBinary {
		if self, err := os.Executable(); err == nil {
			self, _ = filepath.EvalSymlinks(self)
			if strings.HasPrefix(self, filepath.Join(home, ".local", "bin")+"/") {
				remove(self)
			} else if !strings.HasPrefix(self, "/usr/") {
				fmt.Println("the roundbar binary is still at", self)
			}
		}
	}
	fmt.Println(`
roundbar is uninstalled. Your settings are still in ~/.config/roundbar
(delete that folder too if you don't want them).`)
	return nil
}
