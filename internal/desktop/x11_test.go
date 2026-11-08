//go:build linux

package desktop

import (
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"syscall"
	"testing"
	"time"

	"github.com/jezek/xgb"
	"github.com/jezek/xgb/xproto"
)

func TestParseWMClass(t *testing.T) {
	for _, c := range []struct{ in, inst, class string }{
		{"Navigator\x00firefox\x00", "Navigator", "firefox"},
		{"steam_app_1091500\x00steam_app_1091500\x00", "steam_app_1091500", "steam_app_1091500"},
		{"xterm\x00", "xterm", "xterm"},
	} {
		if i, c2 := parseWMClass([]byte(c.in)); i != c.inst || c2 != c.class {
			t.Errorf("%q: %q %q", c.in, i, c2)
		}
	}
}

// xvfb starts a throwaway X server (never the user's display).
func xvfb(t *testing.T) string {
	t.Helper()
	bin, err := exec.LookPath("Xvfb")
	if err != nil {
		t.Skip("Xvfb not installed")
	}
	for n := 90; n < 110; n++ {
		if _, err := os.Stat(fmt.Sprintf("/tmp/.X11-unix/X%d", n)); err == nil {
			continue
		}
		if _, err := os.Stat(fmt.Sprintf("/tmp/.X%d-lock", n)); err == nil {
			continue
		}
		disp := ":" + strconv.Itoa(n)
		cmd := exec.Command(bin, disp, "-nolisten", "tcp", "-noreset", "-screen", "0", "320x240x24")
		if err := cmd.Start(); err != nil {
			t.Skip(err)
		}
		t.Cleanup(func() { cmd.Process.Signal(syscall.SIGTERM); cmd.Wait() }) // lets it remove its lock and socket
		for i := 0; i < 50; i++ {
			if c, err := xgb.NewConnDisplay(disp); err == nil {
				c.Close()
				return disp
			}
			time.Sleep(100 * time.Millisecond)
		}
		t.Fatal("Xvfb did not start")
	}
	t.Skip("no free display number")
	return ""
}

// fakeWM sets the EWMH properties a window manager would and owns a clipboard.
type fakeWM struct {
	t    *testing.T
	c    *xgb.Conn
	root xproto.Window
	atom func(string) xproto.Atom
}

func (w *fakeWM) setProp(win xproto.Window, name string, typ xproto.Atom, format byte, data []byte) {
	unit := uint32(format / 8)
	xproto.ChangeProperty(w.c, xproto.PropModeReplace, win, w.atom(name), typ, format, uint32(len(data))/unit, data)
}

func (w *fakeWM) windows(name string, ids ...xproto.Window) {
	var b []byte
	for _, id := range ids {
		b = binary.LittleEndian.AppendUint32(b, uint32(id))
	}
	w.setProp(w.root, name, xproto.AtomWindow, 32, b)
}

func (w *fakeWM) client(class, title string) xproto.Window {
	id, _ := xproto.NewWindowId(w.c)
	xproto.CreateWindow(w.c, 0, id, w.root, 0, 0, 10, 10, 0, xproto.WindowClassInputOutput, 0, 0, nil)
	w.setProp(id, "WM_CLASS", xproto.AtomString, 8, []byte(class+"\x00"+class+"\x00"))
	w.setProp(id, "_NET_WM_NAME", w.atom("UTF8_STRING"), 8, []byte(title))
	return id
}

func TestX11(t *testing.T) {
	t.Setenv("DISPLAY", xvfb(t))
	t.Setenv("XAUTHORITY", "/nonexistent")
	testApps(t, map[string]string{"firefox": "[Desktop Entry]\nName=Firefox\nIcon=firefox\nStartupWMClass=firefox\n"})
	c, err := xgb.NewConn()
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	wm := &fakeWM{t: t, c: c, root: xproto.Setup(c).DefaultScreen(c).Root}
	wm.atom = func(n string) xproto.Atom {
		r, err := xproto.InternAtom(c, false, uint16(len(n)), n).Reply()
		if err != nil {
			t.Fatal(err)
		}
		return r.Atom
	}
	// receive _NET_ACTIVE_WINDOW requests like a WM does
	xproto.ChangeWindowAttributes(c, wm.root, xproto.CwEventMask, []uint32{xproto.EventMaskSubstructureRedirect})
	game := wm.client("steam_app_42", "Some Game")
	fox := wm.client("firefox", "A page")
	wm.windows("_NET_CLIENT_LIST", game, fox)
	wm.windows("_NET_CLIENT_LIST_STACKING", game, fox)
	wm.windows("_NET_ACTIVE_WINDOW", game)
	xproto.GetInputFocus(c).Reply() // flush

	rec := newRecorder()
	x, err := newX11(rec)
	if err != nil {
		t.Fatal(err)
	}
	defer x.Close()
	if c := x.Caps(); c != (Caps{Focus: true, Windows: true, Clipboard: true}) {
		t.Fatalf("caps %+v", c)
	}
	if f, _ := x.Focus(); f != (Focus{ID: "steam_app_42", Name: "Some Game", Class: "steam_app_42"}) {
		t.Fatalf("focus %+v", f)
	}

	wm.windows("_NET_ACTIVE_WINDOW", fox)
	if f := wait(t, rec.focusC, "focus"); f != (Focus{ID: "firefox", Name: "Firefox", Class: "firefox"}) {
		t.Fatalf("focus %+v", f)
	}
	ws, _ := x.Windows()
	if len(ws) != 2 || ws[0].App != "firefox" || ws[0].Icon != "firefox" || ws[1].App != "steam_app_42" {
		t.Fatalf("windows %+v", ws)
	}

	// clipboard owned by another client
	owner := wm.client("owner", "")
	xproto.SetSelectionOwner(c, owner, wm.atom("CLIPBOARD"), 0)
	events := make(chan xgb.Event, 16)
	go func() {
		for {
			e, err := c.WaitForEvent()
			if e == nil && err == nil {
				return
			}
			if e != nil {
				events <- e
			}
		}
	}()

	if err := x.Activate(ws[1].ID); err != nil {
		t.Fatal(err)
	}
	var sawActivate bool
	deadline := time.After(3 * time.Second)
	for !sawActivate || len(must(x.Clipboard())) == 0 {
		select {
		case e := <-events:
			switch e := e.(type) {
			case xproto.ClientMessageEvent:
				if e.Type == wm.atom("_NET_ACTIVE_WINDOW") && e.Window == game {
					sawActivate = true
				}
			case xproto.SelectionRequestEvent: // serve "copied on x"
				var prop []byte
				typ := e.Target
				if e.Target == wm.atom("TARGETS") {
					typ = xproto.AtomAtom
					prop = binary.LittleEndian.AppendUint32(nil, uint32(wm.atom("UTF8_STRING")))
					xproto.ChangeProperty(c, xproto.PropModeReplace, e.Requestor, e.Property, typ, 32, 1, prop)
				} else {
					prop = []byte("copied on x")
					xproto.ChangeProperty(c, xproto.PropModeReplace, e.Requestor, e.Property, typ, 8, uint32(len(prop)), prop)
				}
				n := xproto.SelectionNotifyEvent{Time: e.Time, Requestor: e.Requestor, Selection: e.Selection, Target: e.Target, Property: e.Property}
				xproto.SendEvent(c, false, e.Requestor, 0, string(n.Bytes()))
			}
		case <-rec.clipC:
		case <-deadline:
			t.Fatalf("activate %v, clipboard %q", sawActivate, must(x.Clipboard()))
		}
	}
	if l := must(x.Clipboard()); l[0] != "copied on x" {
		t.Fatalf("clipboard %q", l)
	}

	// and the other way: our text pasted into another client
	if err := x.SetClipboard("from the device"); err != nil {
		t.Fatal(err)
	}
	xproto.ConvertSelection(c, owner, wm.atom("CLIPBOARD"), wm.atom("UTF8_STRING"), wm.atom("GOT"), 0)
	for {
		select {
		case e := <-events:
			if n, ok := e.(xproto.SelectionNotifyEvent); ok {
				r, err := xproto.GetProperty(c, true, owner, n.Property, xproto.AtomAny, 0, 1024).Reply()
				if err != nil || string(r.Value) != "from the device" {
					t.Fatalf("pasted %q %v", r.Value, err)
				}
				return
			}
		case <-time.After(3 * time.Second):
			t.Fatal("no SelectionNotify")
		}
	}
}
