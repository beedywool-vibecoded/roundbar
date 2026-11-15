package discord

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWithFlag(t *testing.T) {
	for in, want := range map[string]string{
		"Exec=/usr/lib/vesktop/vesktop":                          "Exec=/usr/lib/vesktop/vesktop " + VesktopFlag,
		"Exec=/usr/bin/vesktop %U":                               "Exec=/usr/bin/vesktop " + VesktopFlag + " %U",
		"Exec=flatpak run --branch=stable dev.vencord.Vesktop %U": "Exec=flatpak run --branch=stable dev.vencord.Vesktop " + VesktopFlag + " %U",
	} {
		got := withFlag(in, true)
		if got != want {
			t.Errorf("on: %q -> %q, want %q", in, got, want)
		}
		if withFlag(got, true) != got {
			t.Errorf("not idempotent: %q", got)
		}
		if back := withFlag(got, false); back != in {
			t.Errorf("off: %q -> %q, want %q", got, back, in)
		}
	}
}

func TestPatchVesktopLaunchers(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local/share"))
	sys := filepath.Join(home, "sys")
	_ = os.MkdirAll(sys, 0o755)
	_ = os.WriteFile(filepath.Join(sys, "vesktop.desktop"), []byte("[Desktop Entry]\nName=Vesktop\nExec=/usr/bin/vesktop %U\n"), 0o644)
	old := systemEntries
	systemEntries = func() []string { return []string{filepath.Join(sys, "vesktop.desktop")} }
	defer func() { systemEntries = old }()

	auto := filepath.Join(home, ".config/autostart/vesktop.desktop")
	_ = os.MkdirAll(filepath.Dir(auto), 0o755)
	_ = os.WriteFile(auto, []byte("[Desktop Entry]\nComment=Vesktop autostart script\nExec=/usr/lib/vesktop/vesktop\n"), 0o644)
	other := filepath.Join(home, ".config/autostart/firefox.desktop")
	_ = os.WriteFile(other, []byte("[Desktop Entry]\nExec=firefox\n"), 0o644)

	if _, err := PatchVesktopLaunchers(true); err != nil {
		t.Fatal(err)
	}
	read := func(p string) string { b, _ := os.ReadFile(p); return string(b) }
	if !strings.Contains(read(auto), "Exec=/usr/lib/vesktop/vesktop "+VesktopFlag) {
		t.Fatalf("autostart: %s", read(auto))
	}
	if read(other) != "[Desktop Entry]\nExec=firefox\n" {
		t.Fatal("touched another autostart entry")
	}
	menu := filepath.Join(home, ".local/share/applications/vesktop.desktop")
	if m := read(menu); !strings.Contains(m, VesktopFlag) || !strings.Contains(m, roundbarKey) {
		t.Fatalf("menu copy: %s", m)
	}
	if ch, _ := PatchVesktopLaunchers(true); len(ch) != 0 {
		t.Fatalf("second run changed %v", ch)
	}
	if _, err := PatchVesktopLaunchers(false); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(read(auto), VesktopFlag) {
		t.Fatal("flag left in autostart")
	}
	if _, err := os.Stat(menu); !os.IsNotExist(err) {
		t.Fatal("our menu copy should be gone")
	}
}

// A menu entry the user customised keeps its name and icon: only the flag
// goes in and out, and it is never deleted.
func TestUserMenuEntryKept(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local/share"))
	sys := filepath.Join(home, "sys", "vesktop.desktop")
	_ = os.MkdirAll(filepath.Dir(sys), 0o755)
	_ = os.WriteFile(sys, []byte("[Desktop Entry]\nName=Vesktop\nExec=/usr/bin/vesktop %U\n"), 0o644)
	old := systemEntries
	systemEntries = func() []string { return []string{sys} }
	defer func() { systemEntries = old }()
	user := filepath.Join(home, ".local/share/applications/vesktop.desktop")
	_ = os.MkdirAll(filepath.Dir(user), 0o755)
	mine := "[Desktop Entry]\nName=Discord\nExec=/usr/bin/vesktop %U\nIcon=/home/me/discord.svg\n"
	_ = os.WriteFile(user, []byte(mine), 0o644)

	_, _ = PatchVesktopLaunchers(true)
	b, _ := os.ReadFile(user)
	if !strings.Contains(string(b), "Name=Discord") || !strings.Contains(string(b), "Icon=/home/me/discord.svg") || !strings.Contains(string(b), VesktopFlag) {
		t.Fatalf("user entry: %s", b)
	}
	_, _ = PatchVesktopLaunchers(false)
	b, _ = os.ReadFile(user)
	if string(b) != mine {
		t.Fatalf("not restored: %q", b)
	}
}

func TestVesktopRunning(t *testing.T) {
	proc := t.TempDir()
	add := func(pid string, args ...string) {
		_ = os.MkdirAll(filepath.Join(proc, pid), 0o755)
		_ = os.WriteFile(filepath.Join(proc, pid, "cmdline"), []byte(strings.Join(args, "\x00")+"\x00"), 0o644)
	}
	add("10", "/usr/lib/vesktop/vesktop")
	add("11", "/usr/lib/vesktop/vesktop", "--type=renderer", VesktopFlag)
	if r, p := vesktopRunning(proc); !r || p {
		t.Fatalf("running %v, port %v", r, p)
	}
	add("12", "/usr/lib/vesktop/vesktop", VesktopFlag)
	if _, p := vesktopRunning(proc); !p {
		t.Fatal("port not seen")
	}
}
