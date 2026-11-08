package desktop

import (
	"encoding/binary"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"golang.org/x/sys/unix"
)

// fakeComp is a scripted compositor speaking just enough of the wire protocol:
// it offers wl_seat, the wlr toplevel manager and ext data control.
type fakeComp struct {
	t   *testing.T
	c   *net.UnixConn
	wmu sync.Mutex

	ids      map[string]uint32 // bound interface -> client id
	dev, src uint32
	fds      []int
	activate chan [2]uint32 // handle, seat
	pasted   chan string    // what our source served
}

const (
	topGame   = 0xff000000
	topFox    = 0xff000001
	offerID   = 0xff000010
	stateActv = 2
)

func (f *fakeComp) send(obj uint32, op uint16, args wlArgs, fd int) {
	f.wmu.Lock()
	defer f.wmu.Unlock()
	msg := wlArgs{}.u(obj).u(uint32(8+len(args))<<16 | uint32(op))
	msg = append(msg, args...)
	var oob []byte
	if fd >= 0 {
		oob = unix.UnixRights(fd)
	}
	if _, _, err := f.c.WriteMsgUnix(msg, oob, nil); err != nil {
		f.t.Log(err)
	}
}

func state(vals ...uint32) wlArgs {
	var arr []byte
	for _, v := range vals {
		arr = binary.LittleEndian.AppendUint32(arr, v)
	}
	a := wlArgs{}.u(uint32(len(arr)))
	return append(a, arr...)
}

func (f *fakeComp) toplevel(id uint32, appID, title string, active bool) {
	f.send(f.ids["zwlr_foreign_toplevel_manager_v1"], 0, wlArgs{}.u(id), -1)
	f.send(id, 0, wlArgs{}.s(title), -1)
	f.send(id, 1, wlArgs{}.s(appID), -1)
	if active {
		f.send(id, 4, state(stateActv), -1)
	} else {
		f.send(id, 4, state(), -1)
	}
	f.send(id, 5, nil, -1)
}

func (f *fakeComp) serve() {
	buf := make([]byte, 0, 4096)
	tmp := make([]byte, 4096)
	oob := make([]byte, 256)
	for {
		n, oobn, _, _, err := f.c.ReadMsgUnix(tmp, oob)
		if oobn > 0 {
			cms, _ := unix.ParseSocketControlMessage(oob[:oobn])
			for _, cm := range cms {
				fds, _ := unix.ParseUnixRights(&cm)
				f.fds = append(f.fds, fds...)
			}
		}
		if err != nil || n == 0 {
			return
		}
		buf = append(buf, tmp[:n]...)
		for len(buf) >= 8 {
			size := int(binary.LittleEndian.Uint32(buf[4:]) >> 16)
			if len(buf) < size {
				break
			}
			f.request(binary.LittleEndian.Uint32(buf), uint16(binary.LittleEndian.Uint32(buf[4:])), &wlDec{b: buf[8:size]})
			buf = buf[size:]
		}
	}
}

func (f *fakeComp) popFD() int {
	fd := f.fds[0]
	f.fds = f.fds[1:]
	return fd
}

func (f *fakeComp) request(obj uint32, op uint16, m *wlDec) {
	switch {
	case obj == 1 && op == 1: // get_registry
		reg := m.u()
		f.ids["wl_registry"] = reg
		f.send(reg, 0, wlArgs{}.u(1).s("wl_seat").u(7), -1)
		f.send(reg, 0, wlArgs{}.u(2).s("zwlr_foreign_toplevel_manager_v1").u(3), -1)
		f.send(reg, 0, wlArgs{}.u(3).s("ext_data_control_manager_v1").u(1), -1)
	case obj == 1 && op == 0: // sync
		f.send(m.u(), 0, wlArgs{}.u(0), -1)
	case obj == f.ids["wl_registry"] && op == 0: // bind
		_, iface, _, id := m.u(), m.s(), m.u(), m.u()
		f.ids[iface] = id
		if iface == "zwlr_foreign_toplevel_manager_v1" {
			f.toplevel(topGame, "steam_app_1091500", "Cyberpunk 2077", true)
			f.toplevel(topFox, "firefox", "Some page", false)
		}
	case obj == f.ids["ext_data_control_manager_v1"] && op == 1: // get_data_device
		f.dev = m.u()
		f.send(f.dev, 0, wlArgs{}.u(offerID), -1)
		f.send(offerID, 0, wlArgs{}.s("text/plain;charset=utf-8"), -1)
		f.send(offerID, 0, wlArgs{}.s("TEXT"), -1)
		f.send(f.dev, 1, wlArgs{}.u(offerID), -1)
	case obj == offerID && op == 0: // receive(mime, fd)
		file := os.NewFile(uintptr(f.popFD()), "w")
		io.WriteString(file, "copied text")
		file.Close()
	case (obj == topGame || obj == topFox) && op == 4: // activate(seat)
		f.activate <- [2]uint32{obj, m.u()}
	case obj == f.ids["ext_data_control_manager_v1"] && op == 0: // create_data_source
		f.src = m.u()
	case obj == f.dev && op == 0: // set_selection(source)
		if m.u() != f.src {
			f.t.Error("set_selection with an unknown source")
		}
		var p [2]int
		unix.Pipe2(p[:], unix.O_CLOEXEC)
		f.send(f.src, 0, wlArgs{}.s("text/plain;charset=utf-8"), p[1])
		unix.Close(p[1])
		go func() {
			r := os.NewFile(uintptr(p[0]), "r")
			b, _ := io.ReadAll(r)
			r.Close()
			f.pasted <- string(b)
		}()
	}
}

func TestWayland(t *testing.T) {
	testApps(t, map[string]string{"firefox": "[Desktop Entry]\nName=Firefox\nIcon=firefox\n"})
	sock := filepath.Join(t.TempDir(), "wayland-test")
	l, err := net.ListenUnix("unix", &net.UnixAddr{Name: sock, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	t.Setenv("WAYLAND_DISPLAY", sock)
	comp := &fakeComp{t: t, ids: map[string]uint32{}, activate: make(chan [2]uint32, 1), pasted: make(chan string, 1)}
	go func() {
		c, err := l.AcceptUnix()
		if err != nil {
			return
		}
		comp.c = c
		comp.serve()
	}()

	rec := newRecorder()
	w, err := newWayland(rec, true)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	if c := w.Caps(); c != (Caps{Focus: true, Windows: true, Clipboard: true}) {
		t.Fatalf("caps %+v", c)
	}
	if f := wait(t, rec.focusC, "focus"); f != (Focus{ID: "steam_app_1091500", Name: "Cyberpunk 2077", Class: "steam_app_1091500"}) {
		t.Fatalf("focus %+v", f)
	}
	ws, _ := w.Windows()
	if len(ws) != 2 || ws[0].App != "steam_app_1091500" || ws[1].App != "firefox" || ws[1].Icon != "firefox" {
		t.Fatalf("windows %+v", ws)
	}

	// focus moves to firefox
	comp.send(topGame, 4, state(), -1)
	comp.send(topGame, 5, nil, -1)
	comp.send(topFox, 4, state(stateActv), -1)
	comp.send(topFox, 5, nil, -1)
	if f := wait(t, rec.focusC, "focus"); f.ID != "firefox" || f.Name != "Firefox" {
		t.Fatalf("focus %+v", f)
	}
	if f, _ := w.Focus(); f.ID != "firefox" {
		t.Fatalf("Focus() %+v", f)
	}
	ws, _ = w.Windows()
	if ws[0].App != "firefox" {
		t.Fatalf("most recent first: %+v", ws)
	}

	if err := w.Activate(ws[1].ID); err != nil {
		t.Fatal(err)
	}
	if a := wait(t, comp.activate, "activate"); a[0] != topGame || a[1] != comp.ids["wl_seat"] {
		t.Fatalf("activate %v", a)
	}

	wait(t, rec.clipC, "clipboard")
	if l, _ := w.Clipboard(); len(l) != 1 || l[0] != "copied text" {
		t.Fatalf("clipboard %q", l)
	}
	if err := w.SetClipboard("from the device"); err != nil {
		t.Fatal(err)
	}
	if s := wait(t, comp.pasted, "paste"); s != "from the device" {
		t.Fatalf("served %q", s)
	}

	comp.send(topGame, 6, nil, -1) // closed
	for i := 0; i < 50; i++ {
		if ws, _ = w.Windows(); len(ws) == 1 {
			break
		}
		<-waitShort()
	}
	if len(ws) != 1 {
		t.Fatalf("closed window still listed: %+v", ws)
	}
}

func TestToplevelOrder(t *testing.T) {
	testApps(t, nil)
	ws := toplevelWindows([]toplevel{{id: 1, appID: "a", seq: -1}, {id: 2, appID: "b", seq: 5}, {id: 3, appID: "c", seq: -4}, {id: 4, appID: "d", seq: 9}})
	got := ""
	for _, w := range ws {
		got += w.App
	}
	if got != "dbca" {
		t.Fatal(got)
	}
}

func TestTextMime(t *testing.T) {
	if textMime([]string{"image/png", "UTF8_STRING", "text/plain"}) != "UTF8_STRING" {
		t.Fatal("prefers UTF-8 text")
	}
	if textMime([]string{"text/plain", secretHint}) != "" {
		t.Fatal("secret kept")
	}
	if textMime([]string{"image/png"}) != "" {
		t.Fatal("image as text")
	}
}
