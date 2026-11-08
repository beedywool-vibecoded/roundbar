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
