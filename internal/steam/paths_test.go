package steam

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSplitGamePath(t *testing.T) {
	for exe, want := range map[string][2]string{
		`D:\SteamLibrary\steamapps\common\Hades\x64\Hades.exe`:                   {`D:\SteamLibrary`, "Hades"},
		`C:\Program Files (x86)\Steam\SteamApps\Common\Portal 2\portal2.exe`:     {`C:\Program Files (x86)\Steam`, "Portal 2"},
		`/home/u/.local/share/Steam/steamapps/common/Celeste/Celeste.bin.x86_64`: {`/home/u/.local/share/Steam`, "Celeste"},
		`C:\Program Files\Mozilla Firefox\firefox.exe`:                           {},
		`D:\SteamLibrary\steamapps\common\`:                                      {},
		`D:\SteamLibrary\steamapps\common\Hades`:                                 {}, // a folder, not a file in one
	} {
		lib, dir, ok := SplitGamePath(exe)
		if ok != (want[0] != "") || lib != want[0] || dir != want[1] {
			t.Errorf("%s: got %q %q %v", exe, lib, dir, ok)
		}
	}
}

func TestVDF(t *testing.T) {
	if got := vdfUnescape(`D:\\SteamLibrary`); got != `D:\SteamLibrary` {
		t.Errorf("unescape: %q", got)
	}
	if got := vdfUnescape(`/home/u/Steam`); got != `/home/u/Steam` {
		t.Errorf("unescape: %q", got)
	}
	id, dir := manifestApp([]byte("\"AppState\"\n{\n\t\"appid\"\t\t\"1145360\"\n\t\"name\"\t\t\"Hades\"\n\t\"installdir\"\t\t\"Hades\"\n}"))
	if id != "1145360" || dir != "Hades" {
		t.Errorf("manifest: %q %q", id, dir)
	}
	if !samePath(`C:\Program Files (x86)\Steam`, `c:/program files (x86)/steam/`) || samePath("/a/b", "/a/B") {
		t.Error("samePath")
	}
}

func TestAppForExe(t *testing.T) {
	lib := t.TempDir()
	_ = os.MkdirAll(filepath.Join(lib, "steamapps", "common", "Hades", "x64"), 0o755)
	_ = os.WriteFile(filepath.Join(lib, "steamapps", "appmanifest_1145360.acf"),
		[]byte("\"AppState\"\n{\n\t\"appid\"\t\t\"1145360\"\n\t\"installdir\"\t\t\"Hades\"\n}"), 0o644)
	exe := filepath.Join(lib, "steamapps", "common", "hades", "x64", "Hades.exe")
	if got := AppForExe(exe); got != "1145360" {
		t.Errorf("got %q", got)
	}
	if got := AppForExe(filepath.Join(lib, "steamapps", "common", "Other", "a.exe")); got != "" {
		t.Errorf("got %q", got)
	}
}
