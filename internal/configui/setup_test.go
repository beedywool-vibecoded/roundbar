package configui

import (
	"encoding/json"
	"errors"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/beedywool-vibecoded/roundbar/internal/discord"
	"github.com/beedywool-vibecoded/roundbar/internal/icons"
	"github.com/beedywool-vibecoded/roundbar/internal/udev"
)

func TestUIState(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	s := &server{}
	rec := httptest.NewRecorder()
	s.getUI(rec, httptest.NewRequest("GET", "/api/ui", nil))
	if strings.TrimSpace(rec.Body.String()) != `{"setupDone":false}` {
		t.Fatalf("fresh: %s", rec.Body)
	}
	rec = httptest.NewRecorder()
	s.putUI(rec, httptest.NewRequest("PUT", "/api/ui", strings.NewReader(`{"setupDone":true}`)))
	if rec.Code != 200 {
		t.Fatalf("put: %d %s", rec.Code, rec.Body)
	}
	if !loadUI().SetupDone {
		t.Fatal("not stored")
	}
	// fields left out keep their value
	rec = httptest.NewRecorder()
	s.putUI(rec, httptest.NewRequest("PUT", "/api/ui", strings.NewReader(`{}`)))
	if !loadUI().SetupDone {
		t.Fatal("merge lost setupDone")
	}
	rec = httptest.NewRecorder()
	s.putUI(rec, httptest.NewRequest("PUT", "/api/ui", strings.NewReader(`nope`)))
	if rec.Code != 400 {
		t.Fatal("expected 400")
	}
}

func TestUdevEndpoint(t *testing.T) {
	oldRun, oldInst := udev.Pkexec, udevInstalled
	defer func() { udev.Pkexec, udevInstalled = oldRun, oldInst }()
	ran := 0
	udev.Pkexec = func() error { ran++; return nil }
	s := &server{}

	udevInstalled = func() bool { return true }
	rec := httptest.NewRecorder()
	s.postUdev(rec, httptest.NewRequest("POST", "/api/udev", nil))
	if ran != 0 || !strings.Contains(rec.Body.String(), `"already":true`) {
		t.Fatalf("already installed: ran=%d %s", ran, rec.Body)
	}

	udevInstalled = func() bool { return false }
	rec = httptest.NewRecorder()
	s.postUdev(rec, httptest.NewRequest("POST", "/api/udev", nil))
	if ran != 1 || rec.Code != 200 {
		t.Fatalf("install: ran=%d %d %s", ran, rec.Code, rec.Body)
	}

	udev.Pkexec = func() error { return errors.New("boom") }
	rec = httptest.NewRecorder()
	s.postUdev(rec, httptest.NewRequest("POST", "/api/udev", nil))
	if rec.Code != 500 || !strings.Contains(rec.Body.String(), "boom") {
		t.Fatalf("failure: %d %s", rec.Code, rec.Body)
	}
}

func TestApps(t *testing.T) {
	dir := t.TempDir()
	old, oldMango := icons.AppDirs, mangohudInstalled
	defer func() { icons.AppDirs, mangohudInstalled = old, oldMango }()
	icons.AppDirs = func() []string { return []string{dir} }
	mangohudInstalled = func() bool { return true }
	write := func(name, body string) {
		_ = os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644)
	}
	write("org.mozilla.firefox.desktop", "[Desktop Entry]\nType=Application\nName=Firefox\nIcon=firefox\nExec=firefox %u\nCategories=Network;WebBrowser;\nStartupWMClass=Firefox\n[Desktop Action new]\nName=New window\n")
	write("hidden.desktop", "[Desktop Entry]\nType=Application\nName=Hidden\nNoDisplay=true\n")
	write("link.desktop", "[Desktop Entry]\nType=Link\nName=Link\n")
	write("com.valvesoftware.steam.desktop", "[Desktop Entry]\nType=Application\nName=Steam\nExec=flatpak run com.valvesoftware.Steam\n")
	s := &server{}
	rec := httptest.NewRecorder()
	s.getApps(rec, httptest.NewRequest("GET", "/api/apps", nil))
	var got struct {
		Apps  []icons.App
		Tools map[string]bool
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Apps) != 2 || !got.Tools["mangohud"] {
		t.Fatalf("got %+v", got)
	}
	ff := got.Apps[0]
	if ff.ID != "org.mozilla.firefox" || ff.Name != "Firefox" || ff.Icon != "/api/apps/org.mozilla.firefox/icon" ||
		strings.Join(ff.Categories, ",") != "Network,WebBrowser" || strings.Join(ff.Keys, ",") != "firefox" {
		t.Fatalf("firefox %+v", ff)
	}
	if st := got.Apps[1]; st.ID != "com.valvesoftware.steam" || st.Icon != "" || len(st.Keys) != 0 {
		t.Fatalf("steam %+v", st)
	}
	rec = httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/apps/nope/icon", nil)
	r.SetPathValue("id", "nope")
	s.getAppIcon(rec, r)
	if rec.Code != 404 {
		t.Fatalf("icon: %d", rec.Code)
	}
}

func TestVesktopEndpoint(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	old := vesktopInstalls
	defer func() { vesktopInstalls = old }()
	flat := discord.VesktopFlagsPath("flatpak")
	vesktopInstalls = func() []discord.VesktopInstall {
		return []discord.VesktopInstall{{Kind: "flatpak", Path: flat, Enabled: discord.VesktopFlagEnabled(flat)}}
	}
	s := &server{}
	post := func(body string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		s.postVesktop(rec, httptest.NewRequest("POST", "/api/vesktop", strings.NewReader(body)))
		return rec
	}
	if rec := post(`{}`); rec.Code != 400 {
		t.Fatal("enable is required")
	}
	if rec := post(`{"enable":true}`); rec.Code != 200 || !strings.Contains(rec.Body.String(), `"changed":true`) || !discord.VesktopFlagEnabled(flat) {
		t.Fatalf("enable: %s", rec.Body)
	}
	if rec := post(`{"enable":true}`); !strings.Contains(rec.Body.String(), `"changed":false`) {
		t.Fatalf("idempotent: %s", rec.Body)
	}
	if _, err := os.Stat(discord.VesktopFlagsPath("native")); err == nil {
		t.Fatal("only the installed Vesktop's file is written")
	}
	if rec := post(`{"enable":false}`); !strings.Contains(rec.Body.String(), `"changed":true`) || discord.VesktopFlagEnabled(flat) {
		t.Fatalf("disable: %s", rec.Body)
	}
	rec := httptest.NewRecorder()
	s.getVesktop(rec, httptest.NewRequest("GET", "/api/vesktop", nil))
	if !strings.Contains(rec.Body.String(), `"port":9337`) || !strings.Contains(rec.Body.String(), `"enabled":false`) {
		t.Fatalf("get: %s", rec.Body)
	}
}
