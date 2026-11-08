package discord

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestVesktopFlagBothPaths(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	native := VesktopFlagsPath("native")
	flat := VesktopFlagsPath("flatpak")
	if native != filepath.Join(home, ".config", "vesktop-flags.conf") {
		t.Fatalf("native path %s", native)
	}
	if flat != filepath.Join(home, ".var/app/dev.vencord.Vesktop/config/vesktop-flags.conf") {
		t.Fatalf("flatpak path %s", flat)
	}

	// native: an existing file with the user's own flags
	_ = os.MkdirAll(filepath.Dir(native), 0o755)
	orig := "--enable-features=UseOzonePlatform\n--ozone-platform=wayland\n"
	_ = os.WriteFile(native, []byte(orig), 0o644)
	if ch, err := EnableVesktopFlag(native); err != nil || !ch {
		t.Fatalf("enable: %v %v", ch, err)
	}
	if ch, _ := EnableVesktopFlag(native); ch {
		t.Fatal("second enable should change nothing")
	}
	raw, _ := os.ReadFile(native)
	want := orig + vesktopComment + "\n--remote-debugging-port=9337\n"
	if string(raw) != want || !VesktopFlagEnabled(native) {
		t.Fatalf("got %q", raw)
	}
	if bak, _ := os.ReadFile(native + ".roundbar.bak"); string(bak) != orig {
		t.Fatalf("backup %q", bak)
	}
	if ch, err := DisableVesktopFlag(native); err != nil || !ch {
		t.Fatalf("disable: %v %v", ch, err)
	}
	if raw, _ := os.ReadFile(native); string(raw) != orig || VesktopFlagEnabled(native) {
		t.Fatalf("after disable %q", raw)
	}
	if ch, _ := DisableVesktopFlag(native); ch {
		t.Fatal("second disable should change nothing")
	}

	// flatpak: no file (nor folder) yet
	if ch, err := EnableVesktopFlag(flat); err != nil || !ch {
		t.Fatalf("flatpak enable: %v %v", ch, err)
	}
	if raw, _ := os.ReadFile(flat); string(raw) != vesktopComment+"\n--remote-debugging-port=9337\n" {
		t.Fatalf("flatpak %q", raw)
	}
	if _, err := os.Stat(flat + ".roundbar.bak"); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("no backup for a file that didn't exist")
	}
	if ch, _ := DisableVesktopFlag(flat); !ch {
		t.Fatal("flatpak disable")
	}
	if raw, _ := os.ReadFile(flat); len(raw) != 0 {
		t.Fatalf("flatpak after disable %q", raw)
	}
	// a missing file: nothing to turn off
	if ch, err := DisableVesktopFlag(filepath.Join(home, "none.conf")); ch || err != nil {
		t.Fatal("missing file")
	}
}

func TestVesktopInstalls(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	old := lookPath
	lookPath = func(string) (string, error) { return "", errors.New("no") }
	defer func() { lookPath = old }()
	_ = os.MkdirAll(filepath.Join(home, ".var/app/dev.vencord.Vesktop"), 0o755)
	got := VesktopInstalls()
	if len(got) != 1 || got[0].Kind != "flatpak" || got[0].Enabled {
		// a system-wide native Vesktop on the test machine would add one
		if len(got) != 2 {
			t.Fatalf("got %+v", got)
		}
	}
}
