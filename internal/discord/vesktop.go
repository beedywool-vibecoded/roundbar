package discord

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Vesktop reads extra command line flags from vesktop-flags.conf. Live voice
// needs its DevTools port (CDPPort, localhost only); turning it on appends our
// two lines, turning it off removes exactly those, and every other line stays.
const (
	vesktopComment = "# Roundbar: Discord voice on the device (localhost-only DevTools port)"
	FlatpakVesktop = "dev.vencord.Vesktop"
)

// VesktopFlag is the line Roundbar adds.
var VesktopFlag = fmt.Sprintf("--remote-debugging-port=%d", CDPPort)

// VesktopInstall is one Vesktop on this PC and its flags file.
type VesktopInstall struct {
	Kind    string `json:"kind"` // "native" | "flatpak"
	Path    string `json:"path"`
	Enabled bool   `json:"enabled"`
}

func configHome() string {
	if d := os.Getenv("XDG_CONFIG_HOME"); d != "" {
		return d
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config")
}

// VesktopFlagsPath is the flags file of the native ("native") or Flatpak
// ("flatpak") Vesktop.
func VesktopFlagsPath(kind string) string {
	if kind == "flatpak" {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".var", "app", FlatpakVesktop, "config", "vesktop-flags.conf")
	}
	return filepath.Join(configHome(), "vesktop-flags.conf")
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// lookPath is exec.LookPath, replaceable in tests.
var lookPath = exec.LookPath

// VesktopInstalls lists the Vesktops found (native: its config folder, binary
// or desktop entry; Flatpak: its app folder or exported desktop entry).
func VesktopInstalls() []VesktopInstall {
	home, _ := os.UserHomeDir()
	var out []VesktopInstall
	_, binErr := lookPath("vesktop")
	if exists(filepath.Join(configHome(), "vesktop")) || binErr == nil ||
		exists("/usr/share/applications/vesktop.desktop") || exists(filepath.Join(home, ".local/share/applications/vesktop.desktop")) {
		out = append(out, VesktopInstall{Kind: "native"})
	}
	if exists(filepath.Join(home, ".var", "app", FlatpakVesktop)) ||
		exists("/var/lib/flatpak/exports/share/applications/"+FlatpakVesktop+".desktop") ||
		exists(filepath.Join(home, ".local/share/flatpak/exports/share/applications", FlatpakVesktop+".desktop")) {
		out = append(out, VesktopInstall{Kind: "flatpak"})
	}
	for i := range out {
		out[i].Path = VesktopFlagsPath(out[i].Kind)
		out[i].Enabled = VesktopFlagEnabled(out[i].Path)
	}
	return out
}

func readLines(path string) ([]string, []byte, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	s := strings.TrimSuffix(string(raw), "\n")
	if s == "" {
		return nil, raw, nil
	}
	return strings.Split(s, "\n"), raw, nil
}

// VesktopFlagEnabled: the file has the DevTools port flag.
func VesktopFlagEnabled(path string) bool {
	lines, _, _ := readLines(path)
	for _, l := range lines {
		if strings.TrimSpace(l) == VesktopFlag {
			return true
		}
	}
	return false
}

// write keeps the first version of an existing file as .roundbar.bak.
func write(path string, raw []byte, lines []string) error {
	if raw != nil {
		bak := path + ".roundbar.bak"
		if !exists(bak) {
			if err := os.WriteFile(bak, raw, 0o644); err != nil {
				return err
			}
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	out := ""
	if len(lines) > 0 {
		out = strings.Join(lines, "\n") + "\n"
	}
	return os.WriteFile(path, []byte(out), 0o644)
}

// EnableVesktopFlag appends the flag unless it is there. changed is false when
// nothing had to be written.
func EnableVesktopFlag(path string) (changed bool, err error) {
	lines, raw, err := readLines(path)
	if err != nil {
		return false, err
	}
	for _, l := range lines {
		if strings.TrimSpace(l) == VesktopFlag {
			return false, nil
		}
	}
	return true, write(path, raw, append(lines, vesktopComment, VesktopFlag))
}

// DisableVesktopFlag removes our comment and flag lines, nothing else.
func DisableVesktopFlag(path string) (changed bool, err error) {
	lines, raw, err := readLines(path)
	if err != nil || raw == nil {
		return false, err
	}
	kept := lines[:0:0]
	for _, l := range lines {
		if t := strings.TrimSpace(l); t == VesktopFlag || t == vesktopComment {
			continue
		}
		kept = append(kept, l)
	}
	if len(kept) == len(lines) {
		return false, nil
	}
	return true, write(path, raw, kept)
}

// ---- launchers ------------------------------------------------------------------
//
// vesktop-flags.conf is read only by Arch's /usr/bin/vesktop wrapper: Vesktop
// started by its own autostart entry (it points at the real binary), or
// installed as a .deb, AppImage or Flatpak never sees it. So the flag also goes
// into the launchers the desktop uses: the autostart entry, edited in place,
// and the app-menu entry, through a per-user copy in ~/.local/share/applications
// that takes priority over the system one. Turning it off takes it out again.

const roundbarKey = "X-Roundbar-Copy=true" // marks a menu entry Roundbar created

func dataHome() string {
	if d := os.Getenv("XDG_DATA_HOME"); d != "" {
		return d
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share")
}

// systemEntries are where Vesktop's own menu entries may be.
var systemEntries = func() []string {
	home, _ := os.UserHomeDir()
	return []string{
		"/usr/share/applications/vesktop.desktop",
		"/usr/local/share/applications/vesktop.desktop",
		"/var/lib/flatpak/exports/share/applications/" + FlatpakVesktop + ".desktop",
		filepath.Join(home, ".local/share/flatpak/exports/share/applications", FlatpakVesktop+".desktop"),
	}
}

// withFlag adds or removes the flag in every Exec line of a desktop entry.
func withFlag(entry string, on bool) string {
	lines := strings.Split(entry, "\n")
	for i, l := range lines {
		if !strings.HasPrefix(l, "Exec=") {
			continue
		}
		f := strings.Fields(strings.TrimPrefix(l, "Exec="))
		kept := f[:0:0]
		for _, w := range f {
			if w != VesktopFlag {
				kept = append(kept, w)
			}
		}
		if on && len(kept) > 0 {
			at := 1 // right after the program
			for j, w := range kept {
				if w == FlatpakVesktop { // flatpak run … dev.vencord.Vesktop
					at = j + 1
				}
			}
			kept = append(kept[:at], append([]string{VesktopFlag}, kept[at:]...)...)
		}
		lines[i] = "Exec=" + strings.Join(kept, " ")
	}
	return strings.Join(lines, "\n")
}

func isVesktopEntry(s string) bool {
	for _, l := range strings.Split(s, "\n") {
		if strings.HasPrefix(l, "Exec=") && (strings.Contains(l, "vesktop") || strings.Contains(l, FlatpakVesktop)) {
			return true
		}
	}
	return false
}

// PatchVesktopLaunchers puts the flag into (on) or takes it out of (off)
// Vesktop's autostart entry and its per-user menu entry. It returns the files
// it changed.
func PatchVesktopLaunchers(on bool) ([]string, error) {
	var changed []string
	edit := func(path, before, after string) error {
		if after == before {
			return nil
		}
		changed = append(changed, path)
		return write(path, []byte(before), strings.Split(strings.TrimSuffix(after, "\n"), "\n"))
	}
	// autostart entries (Vesktop writes vesktop.desktop itself)
	auto, _ := filepath.Glob(filepath.Join(configHome(), "autostart", "*.desktop"))
	for _, p := range auto {
		raw, err := os.ReadFile(p)
		if err != nil || !isVesktopEntry(string(raw)) {
			continue
		}
		if err := edit(p, string(raw), withFlag(string(raw), on)); err != nil {
			return changed, err
		}
	}
	// menu entries: a per-user copy shadows the system one
	for _, sys := range systemEntries() {
		raw, err := os.ReadFile(sys)
		if err != nil {
			continue
		}
		user := filepath.Join(dataHome(), "applications", filepath.Base(sys))
		cur, err := os.ReadFile(user)
		switch {
		case err == nil && on:
			if err := edit(user, string(cur), withFlag(string(cur), true)); err != nil {
				return changed, err
			}
		case err == nil && !on && strings.Contains(string(cur), roundbarKey):
			if err := os.Remove(user); err != nil { // our copy: remove it, the system entry is back
				return changed, err
			}
			changed = append(changed, user)
		case err == nil && !on:
			if err := edit(user, string(cur), withFlag(string(cur), false)); err != nil {
				return changed, err
			}
		case on:
			s := withFlag(string(raw), true)
			s = strings.Replace(s, "[Desktop Entry]", "[Desktop Entry]\n"+roundbarKey, 1)
			if err := os.MkdirAll(filepath.Dir(user), 0o755); err != nil {
				return changed, err
			}
			if err := os.WriteFile(user, []byte(s), 0o644); err != nil {
				return changed, err
			}
			changed = append(changed, user)
		}
	}
	return changed, nil
}

// VesktopRunning reports whether Vesktop runs, and whether with the DevTools
// port (a running Vesktop only picks the flag up when restarted).
func VesktopRunning() (running, withPort bool) {
	return vesktopRunning("/proc")
}

func vesktopRunning(proc string) (running, withPort bool) {
	ents, _ := os.ReadDir(proc)
	for _, e := range ents {
		raw, err := os.ReadFile(filepath.Join(proc, e.Name(), "cmdline"))
		if err != nil || len(raw) == 0 {
			continue
		}
		args := strings.Split(strings.TrimRight(string(raw), "\x00"), "\x00")
		exe := filepath.Base(args[0])
		if exe != "vesktop" && !strings.Contains(string(raw), FlatpakVesktop) {
			continue
		}
		main := true
		for _, a := range args[1:] {
			if strings.HasPrefix(a, "--type=") { // renderer, zygote, gpu… helpers
				main = false
			}
		}
		if !main {
			continue
		}
		running = true
		for _, a := range args[1:] {
			if a == VesktopFlag {
				withPort = true
			}
		}
	}
	return running, withPort
}
