//go:build linux

package desktop

import (
	"encoding/json"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/godbus/dbus/v5"
)

// fakeKWin records the scripts it is asked to load.
type fakeKWin struct {
	mu      sync.Mutex
	scripts map[string]string // plugin -> source
	started int
}

func (k *fakeKWin) IsScriptLoaded(plugin string) (bool, *dbus.Error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	_, ok := k.scripts[plugin]
	return ok, nil
}

func (k *fakeKWin) UnloadScript(plugin string) (bool, *dbus.Error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	delete(k.scripts, plugin)
	return true, nil
}

func (k *fakeKWin) LoadScript(path, plugin string) (int32, *dbus.Error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return -1, dbus.MakeFailedError(err)
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	k.scripts[plugin] = string(b)
	return int32(len(k.scripts)), nil
}

func (k *fakeKWin) Start() *dbus.Error {
	k.mu.Lock()
	k.started++
	k.mu.Unlock()
	return nil
}

func (k *fakeKWin) script(plugin string) string {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.scripts[plugin]
}

func TestKDE(t *testing.T) {
	privateBus(t)
	testApps(t, map[string]string{"org.kde.dolphin": "[Desktop Entry]\nName=Dolphin\nIcon=system-file-manager\n"})
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())

	kw, err := dbus.ConnectSessionBus()
	if err != nil {
		t.Fatal(err)
	}
	defer kw.Close()
	fake := &fakeKWin{scripts: map[string]string{}}
	// godbus exports Go method names as-is; KWin's are lowerCamelCase.
	if err := kw.ExportMethodTable(map[string]any{
		"isScriptLoaded": fake.IsScriptLoaded, "unloadScript": fake.UnloadScript,
		"loadScript": fake.LoadScript, "start": fake.Start,
	}, "/Scripting", kwinScript); err != nil {
		t.Fatal(err)
	}
	if _, err := kw.RequestName(kwinName, 0); err != nil {
		t.Fatal(err)
	}

	bus, err := dbus.ConnectSessionBus()
	if err != nil {
		t.Fatal(err)
	}
	defer bus.Close()
	rec := newRecorder()
	k, err := newKDE(bus, rec)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(fake.script(kwinPlugin), "windowActivated") || fake.started != 1 {
		t.Fatal("main script not loaded and started")
	}

	daemon := kw.Object(daemonName, daemonPath)
	wins, _ := json.Marshal([]kdeWindow{
		{ID: "{aaa-1}", App: "org.kde.dolphin", Class: "dolphin", Name: "dolphin", Title: "Home"},
		{ID: "{aaa-2}", App: "", Class: "steam_app_42", Name: "steam_app_42", Title: "Game"},
	})
	if err := daemon.Call(daemonName+".Windows", 0, string(wins)).Err; err != nil {
		t.Fatal(err)
	}
	wait(t, rec.winC, "windows")
	if err := daemon.Call(daemonName+".Focus", 0, "{aaa-2}", "", "steam_app_42", "steam_app_42", "Game").Err; err != nil {
		t.Fatal(err)
	}
	if f := wait(t, rec.focusC, "focus"); f != (Focus{ID: "steam_app_42", Name: "Game", Class: "steam_app_42"}) {
		t.Fatalf("%+v", f)
	}
	ws, _ := k.Windows()
	if len(ws) != 2 || ws[0].ID != "{aaa-2}" || ws[1].App != "org.kde.dolphin" || ws[1].Icon != "system-file-manager" {
		t.Fatalf("%+v", ws)
	}

	// only KWin may report focus
	other, _ := dbus.ConnectSessionBus()
	defer other.Close()
	if err := other.Object(daemonName, daemonPath).Call(daemonName+".Focus", 0, "x", "", "evil", "evil", "").Err; err == nil {
		t.Fatal("call from a stranger accepted")
	}

	if err := k.Activate("{aaa-1}"); err != nil {
		t.Fatal(err)
	}
	if s := fake.script("roundbar-activate-1"); !strings.Contains(s, `"{aaa-1}"`) {
		t.Fatalf("activation script: %q", s)
	}
	if err := k.Activate(`"); evil(); ("`); err == nil {
		t.Fatal("id injected into a script")
	}
	k.Close()
	if fake.script(kwinPlugin) != "" {
		t.Fatal("script left loaded")
	}
}

func TestKlipper(t *testing.T) {
	privateBus(t)
	kl, err := dbus.ConnectSessionBus()
	if err != nil {
		t.Fatal(err)
	}
	defer kl.Close()
	history := []string{"newest", "older", "newest"}
	set := make(chan string, 1)
	kl.ExportMethodTable(map[string]any{
		"getClipboardHistoryMenu": func() ([]string, *dbus.Error) { return history, nil },
		"setClipboardContents":    func(s string) *dbus.Error { set <- s; return nil },
	}, klipperPath, klipperIfc)
	if _, err := kl.RequestName(klipperName, 0); err != nil {
		t.Fatal(err)
	}
	bus, _ := dbus.ConnectSessionBus()
	defer bus.Close()
	rec := newRecorder()
	c := (&kde{conn: bus, ev: rec}).klipper()
	if c == nil {
		t.Fatal("klipper not found")
	}
	defer c.Close()
	if l, _ := c.Clipboard(); len(l) != 2 || l[0] != "newest" || l[1] != "older" {
		t.Fatalf("%q", l)
	}
	kl.Emit(klipperPath, klipperIfc+".clipboardHistoryUpdated")
	wait(t, rec.clipC, "clipboard change")
	if err := c.SetClipboard("x"); err != nil || <-set != "x" {
		t.Fatal(err)
	}
}
