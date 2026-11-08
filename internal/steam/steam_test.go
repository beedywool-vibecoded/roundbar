package steam

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAppID(t *testing.T) {
	if AppID("steam_app_1091500") != "1091500" || AppID("firefox") != "" || AppID("STEAM_APP_42") != "42" {
		t.Fatal("AppID")
	}
}

// Uses the local Steam install when there is one.
func TestLocal(t *testing.T) {
	for _, id := range []string{"228980", "1070560"} {
		if n := Name(id); n != "" {
			t.Logf("%s: %q art=%q", id, n, Art(id))
		}
	}
}

func TestGamePIDs(t *testing.T) {
	proc := t.TempDir()
	write := func(pid, env string) {
		_ = os.MkdirAll(filepath.Join(proc, pid), 0o755)
		_ = os.WriteFile(filepath.Join(proc, pid, "environ"), []byte(env), 0o644)
	}
	write("100", "HOME=/h\x00SteamAppId=1091500\x00")
	write("101", "SteamGameId=1091500\x00")
	write("102", "SteamAppId=10915001\x00") // another game
	write("103", "SteamAppId=0\x00")        // Steam client helpers
	write("self", "SteamAppId=1091500\x00") // not a pid
	got := gamePIDs(proc, "1091500")
	if len(got) != 2 || got[0] != 100 || got[1] != 101 {
		t.Fatalf("got %v", got)
	}
	if gamePIDs(proc, "0") != nil || gamePIDs(proc, "") != nil {
		t.Fatal("app 0 must never match")
	}
}
