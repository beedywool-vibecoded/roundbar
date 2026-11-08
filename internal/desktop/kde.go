package desktop

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"
)

// kde loads a KWin script (kwin/roundbar.js) that calls back into an object
// the daemon exports, dev.roundbar.Daemon, whenever the active window or the
// window list changes. Activation runs a one-shot script. Clipboard comes from
// Klipper when it runs.
type kde struct {
	none
	conn *dbus.Conn
	ev   events
	dir  string // where the script files go

	mu    sync.Mutex
	kwin  string // KWin's unique bus name: only it may call us
	focus Focus
	wins  []kdeWindow
	seen  map[string]int // activation order by window id
	seq   int
	shots int
}

type kdeWindow struct {
	ID    string `json:"id"`
	App   string `json:"app"`   // desktopFileName
	Class string `json:"class"` // resourceClass
	Name  string `json:"name"`  // resourceName
	Title string `json:"title"`
}

//go:embed kwin
var kwinFiles embed.FS

const (
	daemonName  = "dev.roundbar.Daemon"
	daemonPath  = "/dev/roundbar/Daemon"
	kwinName    = "org.kde.KWin"
	kwinScript  = "org.kde.kwin.Scripting"
	kwinPlugin  = "roundbar"
	klipperName = "org.kde.klipper"
	klipperPath = "/klipper"
	klipperIfc  = "org.kde.klipper.klipper"
)

func newKDE(conn *dbus.Conn, ev events) (*kde, error) {
	k := &kde{conn: conn, ev: ev, dir: runtimeDir(), seen: map[string]int{}}
	if err := conn.BusObject().Call("org.freedesktop.DBus.GetNameOwner", 0, kwinName).Store(&k.kwin); err != nil {
		return nil, fmt.Errorf("KWin isn't on the session bus: %w", err)
	}
	if err := conn.Export(kdeObj{k}, daemonPath, daemonName); err != nil {
		return nil, err
	}
	if r, err := conn.RequestName(daemonName, dbus.NameFlagDoNotQueue); err != nil || r != dbus.RequestNameReplyPrimaryOwner {
		conn.Export(nil, daemonPath, daemonName)
		return nil, fmt.Errorf("can't own %s (another daemon running?) %v", daemonName, err)
	}
	src, _ := kwinFiles.ReadFile("kwin/roundbar.js")
	if err := k.run(kwinPlugin, src); err != nil {
		k.Close()
		return nil, err
	}
	return k, nil
}

func (k *kde) scripting() dbus.BusObject { return k.conn.Object(kwinName, "/Scripting") }

// run (re)loads a script into KWin and starts it.
func (k *kde) run(plugin string, src []byte) error {
	path := filepath.Join(k.dir, "roundbar-kwin-"+plugin+".js")
	if err := os.WriteFile(path, src, 0o600); err != nil {
		return err
	}
	var loaded bool
	if k.scripting().Call(kwinScript+".isScriptLoaded", 0, plugin).Store(&loaded) == nil && loaded {
		k.scripting().Call(kwinScript+".unloadScript", 0, plugin)
	}
	var id int32
	if err := k.scripting().Call(kwinScript+".loadScript", 0, path, plugin).Store(&id); err != nil {
		return fmt.Errorf("KWin loadScript: %w", err)
	}
	if id < 0 {
		return errors.New("KWin refused the script")
	}
	return k.scripting().Call(kwinScript+".start", 0).Err
}

func (k *kde) unload(plugin string) {
	k.scripting().Call(kwinScript+".unloadScript", 0, plugin)
	os.Remove(filepath.Join(k.dir, "roundbar-kwin-"+plugin+".js"))
}

func (k *kde) Caps() Caps { return Caps{Focus: true, Windows: true} }

func (k *kde) Close() {
	k.unload(kwinPlugin)
	k.conn.ReleaseName(daemonName)
	k.conn.Export(nil, daemonPath, daemonName)
}

// kdeObj is what the KWin script calls.
type kdeObj struct{ k *kde }

func (o kdeObj) Focus(sender dbus.Sender, id, app, class, name, title string) *dbus.Error {
	if !o.k.trusted(sender) {
		return dbus.MakeFailedError(errors.New("not KWin"))
	}
	o.k.onFocus(kdeWindow{ID: id, App: app, Class: class, Name: name, Title: title})
	return nil
}

func (o kdeObj) Windows(sender dbus.Sender, list string) *dbus.Error {
	if !o.k.trusted(sender) {
		return dbus.MakeFailedError(errors.New("not KWin"))
	}
	var ws []kdeWindow
	if err := json.Unmarshal([]byte(list), &ws); err != nil {
		return dbus.MakeFailedError(err)
	}
	o.k.mu.Lock()
	o.k.wins = ws
	o.k.mu.Unlock()
	o.k.ev.windows()
	return nil
}

// trusted: the call comes from KWin (re-checked once if KWin restarted).
func (k *kde) trusted(sender dbus.Sender) bool {
	k.mu.Lock()
	ok := string(sender) == k.kwin
	k.mu.Unlock()
	if ok {
		return true
	}
	var owner string
	if k.conn.BusObject().Call("org.freedesktop.DBus.GetNameOwner", 0, kwinName).Store(&owner) != nil || owner != string(sender) {
		return false
	}
	k.mu.Lock()
	k.kwin = owner
	k.mu.Unlock()
	return true
}

func (k *kde) onFocus(w kdeWindow) {
	f := focusFor(w.App, w.Class, w.Name, w.Title)
	k.mu.Lock()
	k.seq++
	k.seen[w.ID] = k.seq
	same := f == k.focus
	k.focus = f
	k.mu.Unlock()
	if !same {
		k.ev.focus(f)
	}
	k.ev.windows()
}

func (k *kde) Focus() (Focus, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.focus, nil
}

func (k *kde) Windows() ([]Window, error) {
	k.mu.Lock()
	ws := slices.Clone(k.wins)
	seen := make(map[string]int, len(ws))
	for _, w := range ws {
		seen[w.ID] = k.seen[w.ID]
	}
	k.mu.Unlock()
	return kdeWindows(ws, seen), nil
}

// kdeWindows orders KWin's list (oldest first) most recently active first, then newest.
func kdeWindows(ws []kdeWindow, seen map[string]int) []Window {
	slices.Reverse(ws)
	sort.SliceStable(ws, func(i, j int) bool { return seen[ws[i].ID] > seen[ws[j].ID] })
	out := make([]Window, 0, len(ws))
	for _, w := range ws {
		out = append(out, windowFor(w.ID, w.App, w.Class, w.Name, w.Title))
	}
	return out
}

var kwinID = regexp.MustCompile(`^\{?[0-9a-fA-F-]{1,40}\}?$`)

func (k *kde) Activate(id string) error {
	if !kwinID.MatchString(id) {
		return fmt.Errorf("bad window id %q", id)
	}
	src, _ := kwinFiles.ReadFile("kwin/activate.js")
	k.mu.Lock()
	k.shots++
	plugin := fmt.Sprintf("roundbar-activate-%d", k.shots)
	k.mu.Unlock()
	if err := k.run(plugin, []byte(strings.Replace(string(src), `"ROUNDBAR_ID"`, `"`+id+`"`, 1))); err != nil {
		return err
	}
	time.AfterFunc(2*time.Second, func() { k.unload(plugin) }) // it ran on start; drop it
	return nil
}

// ---- Klipper ----

type klipper struct {
	none
	conn *dbus.Conn
	ev   events
	sig  chan *dbus.Signal
}

// klipper returns the clipboard backend if Klipper is on the bus, else nil.
func (k *kde) klipper() Shell {
	var has bool
	if k.conn.BusObject().Call("org.freedesktop.DBus.NameHasOwner", 0, klipperName).Store(&has) != nil || !has {
		return nil
	}
	if k.conn.AddMatchSignal(dbus.WithMatchInterface(klipperIfc), dbus.WithMatchMember("clipboardHistoryUpdated")) != nil {
		return nil
	}
	c := &klipper{conn: k.conn, ev: k.ev, sig: make(chan *dbus.Signal, 8)}
	k.conn.Signal(c.sig)
	go func() {
		for s := range c.sig {
			if s.Name == klipperIfc+".clipboardHistoryUpdated" {
				c.ev.clips()
			}
		}
	}()
	return c
}

func (c *klipper) Caps() Caps { return Caps{Clipboard: true} }

func (c *klipper) Close() { c.conn.RemoveSignal(c.sig) }

func (c *klipper) obj() dbus.BusObject { return c.conn.Object(klipperName, klipperPath) }

func (c *klipper) Clipboard() ([]string, error) {
	var all []string
	if err := c.obj().Call(klipperIfc+".getClipboardHistoryMenu", 0).Store(&all); err != nil {
		return nil, err
	}
	var out []string
	for i := len(all) - 1; i >= 0; i-- { // oldest first, so the newest ends up on top
		out, _ = pushHistory(out, all[i])
	}
	return out, nil
}

func (c *klipper) SetClipboard(text string) error {
	return c.obj().Call(klipperIfc+".setClipboardContents", 0, text).Err
}
