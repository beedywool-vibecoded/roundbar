package desktop

import (
	"encoding/binary"
	"errors"
	"io"
	"log"
	"os"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"golang.org/x/sys/unix"
)

// wayland follows windows on compositors other than GNOME and KDE through
// standard protocols: zwlr_foreign_toplevel_manager_v1 (wlroots compositors,
// Hyprland, niri…: list, focus and activate) or ext_foreign_toplevel_list_v1
// (list only), and the clipboard through ext/zwlr data control.
type wayland struct {
	none
	w  *wlConn
	ev events

	mu      sync.Mutex
	globals map[string]wlGlobal
	seat    uint32
	wlr     bool // wlr toplevel manager (else ext list)
	tops    map[uint32]*toplevel
	seq     int
	last    Focus

	dc     uint32 // data control manager (0: none)
	dev    uint32 // its device for our seat
	offers map[uint32][]string
	sel    uint32 // offer holding the current clipboard
	clips  []string
}

type wlGlobal struct{ name, version uint32 }

type toplevel struct {
	id               uint32
	appID, title     string
	activated, ready bool
	seq              int // when it was last activated (0: never), else when created (negative)
}

const (
	wlrStateActivated = 2
	maxClips          = 10
	maxClipLen        = 2000
	secretHint        = "x-kde-passwordManagerHint"
)

// newWayland connects; toplevels=false binds only the clipboard.
func newWayland(ev events, toplevels bool) (*wayland, error) {
	w, err := wlDial()
	if err != nil {
		return nil, err
	}
	b := &wayland{w: w, ev: ev, globals: map[string]wlGlobal{}, tops: map[uint32]*toplevel{}, offers: map[uint32][]string{}}
	ready := make(chan struct{})
	go func() {
		if err := w.loop(); err != nil && !errors.Is(err, os.ErrClosed) && !strings.Contains(err.Error(), "use of closed") {
			log.Printf("wayland: %v", err)
		}
	}()
	w.on(1, b.display)
	_, _ = w.req(1, 1, func(id uint32) wlArgs { return wlArgs{}.u(id) }, b.registry, -1) // get_registry
	b.sync(func() {
		b.bind(toplevels)
		b.sync(func() { close(ready) }) // initial toplevels and selection are in
	})
	select {
	case <-ready:
	case <-time.After(3 * time.Second):
		w.Close()
		return nil, errors.New("wayland: compositor did not answer")
	}
	if c := b.Caps(); !c.Windows && !c.Clipboard {
		w.Close()
		return nil, errors.New("wayland: the compositor offers no window list or clipboard protocol")
	}
	return b, nil
}

func (b *wayland) Close() { b.w.Close() }

func (b *wayland) Caps() Caps {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.caps()
}

// caps is Caps for callers holding b.mu.
func (b *wayland) caps() Caps {
	_, wlr := b.globals["zwlr_foreign_toplevel_manager_v1"]
	_, ext := b.globals["ext_foreign_toplevel_list_v1"]
	return Caps{Focus: b.wlr, Windows: wlr || ext, Clipboard: b.dev != 0}
}

func (b *wayland) sync(done func()) {
	_, _ = b.w.req(1, 0, func(id uint32) wlArgs { return wlArgs{}.u(id) }, func(op uint16, m *wlDec) { done() }, -1)
}

func (b *wayland) display(op uint16, m *wlDec) {
	if op == 0 { // error(object, code, message)
		obj, code, msg := m.u(), m.u(), m.s()
		log.Printf("wayland: protocol error on object %d (code %d): %s", obj, code, msg)
	}
}

func (b *wayland) registry(op uint16, m *wlDec) {
	if op != 0 { // global_remove: nothing we bind goes away mid-session
		return
	}
	name, iface, ver := m.u(), m.s(), m.u()
	b.mu.Lock()
	if _, dup := b.globals[iface]; !dup {
		b.globals[iface] = wlGlobal{name, ver}
	}
	b.mu.Unlock()
}

// bindGlobal binds a global at min(version, max); 0 if the compositor lacks it.
func (b *wayland) bindGlobal(iface string, max uint32, h wlHandler) uint32 {
	b.mu.Lock()
	g, ok := b.globals[iface]
	b.mu.Unlock()
	if !ok {
		return 0
	}
	v := min(g.version, max)
	id, err := b.w.req(2, 0, func(id uint32) wlArgs { return wlArgs{}.u(g.name).s(iface).u(v).u(id) }, h, -1)
	if err != nil {
		return 0
	}
	return id
}

func (b *wayland) bind(toplevels bool) {
	seat := b.bindGlobal("wl_seat", 1, func(uint16, *wlDec) {})
	if !toplevels {
		b.mu.Lock()
		delete(b.globals, "zwlr_foreign_toplevel_manager_v1")
		delete(b.globals, "ext_foreign_toplevel_list_v1")
		b.mu.Unlock()
	} else if b.bindGlobal("zwlr_foreign_toplevel_manager_v1", 3, b.manager(true)) != 0 {
		b.mu.Lock()
		b.wlr = true
		b.mu.Unlock()
	} else {
		b.bindGlobal("ext_foreign_toplevel_list_v1", 1, b.manager(false))
	}
	if seat == 0 {
		return
	}
	dc := b.bindGlobal("ext_data_control_manager_v1", 1, func(uint16, *wlDec) {})
	if dc == 0 {
		dc = b.bindGlobal("zwlr_data_control_manager_v1", 1, func(uint16, *wlDec) {})
	}
	b.mu.Lock()
	b.seat, b.dc = seat, dc
	b.mu.Unlock()
	if dc != 0 {
		dev, _ := b.w.req(dc, 1, func(id uint32) wlArgs { return wlArgs{}.u(id).u(seat) }, b.device, -1) // get_data_device
		b.mu.Lock()
		b.dev = dev
		b.mu.Unlock()
	}
}

// ---- toplevels ----

// manager handles both managers: event 0 announces a new toplevel handle.
func (b *wayland) manager(wlr bool) wlHandler {
	return func(op uint16, m *wlDec) {
		if op != 0 {
			return
		}
		id := m.u()
		b.mu.Lock()
		b.seq++
		t := &toplevel{id: id, seq: -b.seq}
		b.tops[id] = t
		b.mu.Unlock()
		if wlr {
			b.w.on(id, func(op uint16, m *wlDec) { b.wlrHandle(t, op, m) })
		} else {
			b.w.on(id, func(op uint16, m *wlDec) { b.extHandle(t, op, m) })
		}
	}
}

// wlrHandle: title 0, app_id 1, output_enter 2, output_leave 3, state 4, done 5, closed 6, parent 7.
func (b *wayland) wlrHandle(t *toplevel, op uint16, m *wlDec) {
	b.mu.Lock()
	switch op {
	case 0:
		t.title = m.s()
	case 1:
		t.appID = m.s()
	case 4:
		st := m.arr()
		was := t.activated
		t.activated = false
		for i := 0; i+4 <= len(st); i += 4 {
			if binary.LittleEndian.Uint32(st[i:]) == wlrStateActivated {
				t.activated = true
			}
		}
		if t.activated && !was {
			b.seq++
			t.seq = b.seq
		}
	case 5:
		b.mu.Unlock()
		b.done(t)
		return
	case 6:
		b.mu.Unlock()
		b.closed(t, 7)
		return
	}
	b.mu.Unlock()
}

// extHandle: closed 0, done 1, title 2, app_id 3, identifier 4.
func (b *wayland) extHandle(t *toplevel, op uint16, m *wlDec) {
	switch op {
	case 0:
		b.closed(t, 0)
	case 1:
		b.done(t)
	case 2:
		b.mu.Lock()
		t.title = m.s()
		b.mu.Unlock()
	case 3:
		b.mu.Lock()
		t.appID = m.s()
		b.mu.Unlock()
	}
}

// done applies a toplevel's batch of changes: focus if it is the active one.
func (b *wayland) done(t *toplevel) {
	b.mu.Lock()
	t.ready = true
	var f Focus
	emit := false
	if t.activated {
		f = focusFor("", t.appID, "", t.title)
		emit = f != b.last
		b.last = f
	}
	b.mu.Unlock()
	if emit {
		b.ev.focus(f)
	}
	b.ev.windows()
}

func (b *wayland) closed(t *toplevel, destroyOp uint16) {
	b.mu.Lock()
	delete(b.tops, t.id)
	b.mu.Unlock()
	b.w.on(t.id, nil)
	_, _ = b.w.req(t.id, destroyOp, nil, nil, -1)
	b.ev.windows()
}

func (b *wayland) Focus() (Focus, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.wlr {
		return Focus{}, ErrUnsupported
	}
	for _, t := range b.tops {
		if t.activated {
			return focusFor("", t.appID, "", t.title), nil
		}
	}
	return Focus{}, nil
}

// Windows lists toplevels, most recently activated first, then newest.
func (b *wayland) Windows() ([]Window, error) {
	b.mu.Lock()
	if !b.caps().Windows {
		b.mu.Unlock()
		return nil, ErrUnsupported
	}
	ts := make([]toplevel, 0, len(b.tops))
	for _, t := range b.tops {
		if t.ready {
			ts = append(ts, *t)
		}
	}
	b.mu.Unlock()
	return toplevelWindows(ts), nil
}

func toplevelWindows(ts []toplevel) []Window {
	sort.Slice(ts, func(i, j int) bool {
		a, c := ts[i].seq, ts[j].seq
		if (a > 0) != (c > 0) {
			return a > 0
		}
		if a > 0 {
			return a > c
		}
		return a < c // created later = more negative
	})
	out := make([]Window, 0, len(ts))
	for _, t := range ts {
		out = append(out, windowFor(strconv.FormatUint(uint64(t.id), 10), "", t.appID, "", t.title))
	}
	return out
}

func (b *wayland) Activate(id string) error {
	n, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		return err
	}
	b.mu.Lock()
	_, ok := b.tops[uint32(n)]
	wlr, seat := b.wlr, b.seat
	b.mu.Unlock()
	if !wlr || seat == 0 {
		return ErrUnsupported
	}
	if !ok {
		return errors.New("no such window")
	}
	_, err = b.w.req(uint32(n), 4, func(uint32) wlArgs { return wlArgs{}.u(seat) }, nil, -1)
	return err
}

// ---- clipboard (ext_data_control_v1 / zwlr_data_control_v1, same opcodes) ----

// device: data_offer 0, selection 1, finished 2, primary_selection 3.
func (b *wayland) device(op uint16, m *wlDec) {
	switch op {
	case 0:
		id := m.u()
		b.mu.Lock()
		b.offers[id] = nil
		b.mu.Unlock()
		b.w.on(id, func(op uint16, m *wlDec) {
			if op == 0 { // offer(mime)
				mime := m.s()
				b.mu.Lock()
				if ms, ok := b.offers[id]; ok && len(ms) < 64 {
					b.offers[id] = append(ms, mime)
				}
				b.mu.Unlock()
			}
		})
	case 1:
		id := m.u()
		b.mu.Lock()
		old := b.sel
		b.sel = id
		mimes := b.offers[id]
		b.mu.Unlock()
		if old != 0 && old != id {
			b.dropOffer(old)
		}
		if id != 0 {
			b.readOffer(id, mimes)
		}
	case 2:
		b.mu.Lock()
		b.dev = 0
		b.mu.Unlock()
	case 3:
		if id := m.u(); id != 0 {
			b.mu.Lock()
			keep := id == b.sel
			b.mu.Unlock()
			if !keep {
				b.dropOffer(id)
			}
		}
	}
}

func (b *wayland) dropOffer(id uint32) {
	b.mu.Lock()
	delete(b.offers, id)
	b.mu.Unlock()
	b.w.on(id, nil)
	_, _ = b.w.req(id, 1, nil, nil, -1)
}

// textMime picks the text type to ask for; "" if none or a secret.
func textMime(mimes []string) string {
	if slices.Contains(mimes, secretHint) {
		return "" // password managers mark secrets: never keep them
	}
	for _, want := range []string{"text/plain;charset=utf-8", "UTF8_STRING", "text/plain", "STRING", "TEXT"} {
		if slices.Contains(mimes, want) {
			return want
		}
	}
	return ""
}

func (b *wayland) readOffer(id uint32, mimes []string) {
	mime := textMime(mimes)
	if mime == "" {
		return
	}
	var p [2]int
	if err := unix.Pipe2(p[:], unix.O_CLOEXEC); err != nil {
		return
	}
	_ = unix.SetNonblock(p[0], true)                                                     // pollable, so the read deadline works
	_, err := b.w.req(id, 0, func(uint32) wlArgs { return wlArgs{}.s(mime) }, nil, p[1]) // receive(mime, fd)
	unix.Close(p[1])
	if err != nil {
		unix.Close(p[0])
		return
	}
	go func() { // the source writes at its pace; never block the event loop on it
		f := os.NewFile(uintptr(p[0]), "clipboard")
		defer f.Close()
		_ = f.SetReadDeadline(time.Now().Add(2 * time.Second))
		data, _ := io.ReadAll(io.LimitReader(f, 64<<10))
		_, _ = io.Copy(io.Discard, f)
		b.addClip(string(data))
	}()
}

func (b *wayland) addClip(text string) {
	if b.pushClip(text) {
		b.ev.clips()
	}
}

func (b *wayland) pushClip(text string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	var ok bool
	b.clips, ok = pushHistory(b.clips, text)
	return ok
}

// pushHistory puts text first in a clipboard history (≤ 10, each ≤ 2000 chars).
func pushHistory(list []string, text string) ([]string, bool) {
	text = strings.ToValidUTF8(text, "")
	if utf8.RuneCountInString(text) > maxClipLen {
		text = string([]rune(text)[:maxClipLen])
	}
	if strings.TrimSpace(text) == "" {
		return list, false
	}
	if len(list) > 0 && list[0] == text {
		return list, false
	}
	out := append([]string{text}, slices.DeleteFunc(slices.Clone(list), func(s string) bool { return s == text })...)
	if len(out) > maxClips {
		out = out[:maxClips]
	}
	return out, true
}

func (b *wayland) Clipboard() ([]string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.dev == 0 {
		return nil, ErrUnsupported
	}
	return slices.Clone(b.clips), nil
}

// SetClipboard owns the selection with our own source until someone else copies.
func (b *wayland) SetClipboard(text string) error {
	b.mu.Lock()
	dc, dev := b.dc, b.dev
	b.mu.Unlock()
	if dev == 0 {
		return ErrUnsupported
	}
	var self atomic.Uint32 // the handler can't see src's assignment otherwise
	src, err := b.w.req(dc, 0, func(id uint32) wlArgs { return wlArgs{}.u(id) }, func(op uint16, m *wlDec) {
		switch op {
		case 0: // send(mime, fd)
			_ = m.s()
			if fd := m.fd(); fd >= 0 {
				go func() {
					f := os.NewFile(uintptr(fd), "clipboard")
					_, _ = io.WriteString(f, text)
					f.Close()
				}()
			}
		case 1: // cancelled: someone else owns the clipboard now
			id := self.Load()
			b.w.on(id, nil)
			_, _ = b.w.req(id, 1, nil, nil, -1)
		}
	}, -1)
	if err != nil {
		return err
	}
	self.Store(src)
	for _, mime := range []string{"text/plain;charset=utf-8", "text/plain", "UTF8_STRING", "STRING", "TEXT"} {
		_, _ = b.w.req(src, 0, func(uint32) wlArgs { return wlArgs{}.s(mime) }, nil, -1)
	}
	_, err = b.w.req(dev, 0, func(uint32) wlArgs { return wlArgs{}.u(src) }, nil, -1) // set_selection
	b.addClip(text)
	return err
}
