package desktop

import (
	"encoding/binary"
	"errors"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/jezek/xgb"
	"github.com/jezek/xgb/xfixes"
	"github.com/jezek/xgb/xproto"
)

// x11 follows the EWMH root window properties (_NET_ACTIVE_WINDOW,
// _NET_CLIENT_LIST) through PropertyNotify events, activates windows with the
// _NET_ACTIVE_WINDOW client message and keeps a clipboard history through
// XFixes selection events. One X connection, no other process.
type x11 struct {
	none
	c    *xgb.Conn
	root xproto.Window
	win  xproto.Window // our hidden window: selection requests and transfers
	ev   events
	a    map[string]xproto.Atom
	xfix bool

	mu    sync.Mutex
	last  Focus
	seen  map[xproto.Window]int // activation order
	seq   int
	time  xproto.Timestamp // latest server time seen
	clips []string
	own   string // text we serve while we own CLIPBOARD
}

var x11Atoms = []string{
	"_NET_ACTIVE_WINDOW", "_NET_CLIENT_LIST", "_NET_CLIENT_LIST_STACKING", "_NET_WM_NAME", "UTF8_STRING",
	"_NET_WM_WINDOW_TYPE", "_NET_WM_WINDOW_TYPE_NORMAL", "_NET_WM_STATE", "_NET_WM_STATE_SKIP_TASKBAR",
	"_GTK_APPLICATION_ID", "_KDE_NET_WM_DESKTOP_FILE", "CLIPBOARD", "TARGETS", "INCR", "ROUNDBAR_SEL",
	"text/plain;charset=utf-8", "text/plain", "TEXT", secretHint,
}

func newX11(ev events) (*x11, error) {
	c, err := xgb.NewConn() // $DISPLAY
	if err != nil {
		return nil, err
	}
	x := &x11{c: c, ev: ev, root: xproto.Setup(c).DefaultScreen(c).Root, a: map[string]xproto.Atom{}, seen: map[xproto.Window]int{}}
	cookies := make([]xproto.InternAtomCookie, len(x11Atoms))
	for i, n := range x11Atoms {
		cookies[i] = xproto.InternAtom(c, false, uint16(len(n)), n)
	}
	for i, ck := range cookies {
		r, err := ck.Reply()
		if err != nil {
			c.Close()
			return nil, err
		}
		x.a[x11Atoms[i]] = r.Atom
	}
	if err := xproto.ChangeWindowAttributesChecked(c, x.root, xproto.CwEventMask, []uint32{xproto.EventMaskPropertyChange}).Check(); err != nil {
		c.Close()
		return nil, err
	}
	id, err := xproto.NewWindowId(c)
	if err != nil {
		c.Close()
		return nil, err
	}
	x.win = id
	_ = xproto.CreateWindowChecked(c, 0, id, x.root, -1, -1, 1, 1, 0, xproto.WindowClassInputOnly, 0,
		xproto.CwEventMask, []uint32{xproto.EventMaskPropertyChange}).Check()
	if xfixes.Init(c) == nil {
		if _, err := xfixes.QueryVersion(c, 5, 0).Reply(); err == nil {
			x.xfix = xfixes.SelectSelectionInputChecked(c, x.win, x.a["CLIPBOARD"], xfixes.SelectionEventMaskSetSelectionOwner).Check() == nil
		}
	}
	if _, err := x.prop(x.root, "_NET_ACTIVE_WINDOW", xproto.AtomWindow); err != nil {
		c.Close()
		return nil, errors.New("x11: the window manager doesn't publish _NET_ACTIVE_WINDOW")
	}
	go x.loop()
	return x, nil
}

func (x *x11) Caps() Caps { return Caps{Focus: true, Windows: true, Clipboard: x.xfix} }

func (x *x11) Close() { x.c.Close() }

func (x *x11) loop() {
	for {
		e, err := x.c.WaitForEvent()
		if e == nil && err == nil {
			return // connection closed
		}
		switch e := e.(type) {
		case xproto.PropertyNotifyEvent:
			x.setTime(e.Time)
			if e.Window != x.root {
				continue
			}
			switch e.Atom {
			case x.a["_NET_ACTIVE_WINDOW"]:
				x.activeChanged()
			case x.a["_NET_CLIENT_LIST"], x.a["_NET_CLIENT_LIST_STACKING"]:
				x.ev.windows()
			}
		case xfixes.SelectionNotifyEvent:
			x.setTime(e.Timestamp)
			if e.Owner != x.win && e.Owner != 0 {
				xproto.ConvertSelection(x.c, x.win, e.Selection, x.a["TARGETS"], x.a["ROUNDBAR_SEL"], e.Timestamp)
			}
		case xproto.SelectionNotifyEvent:
			x.selectionNotify(e)
		case xproto.SelectionRequestEvent:
			x.selectionRequest(e)
		}
	}
}

func (x *x11) setTime(t xproto.Timestamp) {
	x.mu.Lock()
	if t > x.time {
		x.time = t
	}
	x.mu.Unlock()
}

// prop reads a whole property (up to 256 KiB).
func (x *x11) prop(w xproto.Window, name string, typ xproto.Atom) ([]byte, error) {
	r, err := xproto.GetProperty(x.c, false, w, x.a[name], typ, 0, 1<<16).Reply()
	if err != nil {
		return nil, err
	}
	if r.Format == 0 {
		return nil, errors.New("x11: no " + name)
	}
	return r.Value, nil
}

func u32s(b []byte) []uint32 {
	out := make([]uint32, 0, len(b)/4)
	for i := 0; i+4 <= len(b); i += 4 {
		out = append(out, binary.LittleEndian.Uint32(b[i:])) // xgb hands values in host order
	}
	return out
}

func (x *x11) active() xproto.Window {
	b, err := x.prop(x.root, "_NET_ACTIVE_WINDOW", xproto.AtomWindow)
	if v := u32s(b); err == nil && len(v) > 0 {
		return xproto.Window(v[0])
	}
	return 0
}

// info reads a window's class, instance, title and desktop id hints.
func (x *x11) info(w xproto.Window) (class, instance, title, desktopID string) {
	if r, err := xproto.GetProperty(x.c, false, w, xproto.AtomWmClass, xproto.AtomString, 0, 256).Reply(); err == nil && r.Format != 0 {
		instance, class = parseWMClass(r.Value)
	}
	if b, err := x.prop(w, "_NET_WM_NAME", x.a["UTF8_STRING"]); err == nil && len(b) > 0 {
		title = string(b)
	} else if b, err := xproto.GetProperty(x.c, false, w, xproto.AtomWmName, xproto.AtomAny, 0, 1<<12).Reply(); err == nil {
		title = string(b.Value)
	}
	for _, p := range []string{"_GTK_APPLICATION_ID", "_KDE_NET_WM_DESKTOP_FILE"} {
		if b, err := x.prop(w, p, x.a["UTF8_STRING"]); err == nil && len(b) > 0 {
			desktopID = string(b)
			break
		}
	}
	return
}

// parseWMClass splits WM_CLASS ("instance\0class\0").
func parseWMClass(b []byte) (instance, class string) {
	parts := strings.Split(strings.TrimRight(string(b), "\x00"), "\x00")
	instance = parts[0]
	if len(parts) > 1 {
		class = parts[1]
	} else {
		class = instance
	}
	return
}

func (x *x11) focusOf(w xproto.Window) Focus {
	if w == 0 || w == x.root {
		return Focus{}
	}
	class, instance, title, id := x.info(w)
	return focusFor(id, class, instance, title)
}

func (x *x11) activeChanged() {
	w := x.active()
	f := x.focusOf(w)
	x.mu.Lock()
	if w != 0 {
		x.seq++
		x.seen[w] = x.seq
	}
	same := f == x.last
	x.last = f
	x.mu.Unlock()
	if !same && f != (Focus{}) {
		x.ev.focus(f)
	}
	x.ev.windows()
}

func (x *x11) Focus() (Focus, error) { return x.focusOf(x.active()), nil }

// Windows lists normal, taskbar-visible clients, most recently active first
// (then by stacking order, top first).
func (x *x11) Windows() ([]Window, error) {
	b, err := x.prop(x.root, "_NET_CLIENT_LIST_STACKING", xproto.AtomWindow)
	if err != nil {
		if b, err = x.prop(x.root, "_NET_CLIENT_LIST", xproto.AtomWindow); err != nil {
			return nil, err
		}
	}
	ids := u32s(b)
	slices.Reverse(ids)
	x.mu.Lock()
	order := make(map[uint32]int, len(ids))
	live := map[xproto.Window]bool{}
	for _, id := range ids {
		order[id] = x.seen[xproto.Window(id)]
		live[xproto.Window(id)] = true
	}
	for w := range x.seen { // forget closed windows
		if !live[w] {
			delete(x.seen, w)
		}
	}
	x.mu.Unlock()
	sort.SliceStable(ids, func(i, j int) bool { return order[ids[i]] > order[ids[j]] })
	out := make([]Window, 0, len(ids))
	for _, id := range ids {
		w := xproto.Window(id)
		if !x.listed(w) {
			continue
		}
		class, instance, title, did := x.info(w)
		out = append(out, windowFor(strconv.FormatUint(uint64(id), 10), did, class, instance, title))
	}
	return out, nil
}

// listed: a normal window (or untyped) that doesn't skip the taskbar.
func (x *x11) listed(w xproto.Window) bool {
	if b, err := x.prop(w, "_NET_WM_WINDOW_TYPE", xproto.AtomAtom); err == nil {
		if t := u32s(b); len(t) > 0 && !slices.Contains(t, uint32(x.a["_NET_WM_WINDOW_TYPE_NORMAL"])) {
			return false
		}
	}
	if b, err := x.prop(w, "_NET_WM_STATE", xproto.AtomAtom); err == nil {
		if slices.Contains(u32s(b), uint32(x.a["_NET_WM_STATE_SKIP_TASKBAR"])) {
			return false
		}
	}
	return true
}

func (x *x11) Activate(id string) error {
	n, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		return err
	}
	x.mu.Lock()
	t := x.time
	x.mu.Unlock()
	ev := xproto.ClientMessageEvent{Format: 32, Window: xproto.Window(n), Type: x.a["_NET_ACTIVE_WINDOW"],
		Data: xproto.ClientMessageDataUnionData32New([]uint32{2, uint32(t), 0, 0, 0})} // 2: from a pager
	return xproto.SendEventChecked(x.c, false, x.root,
		xproto.EventMaskSubstructureNotify|xproto.EventMaskSubstructureRedirect, string(ev.Bytes())).Check()
}

// ---- clipboard ----

// selectionNotify continues a transfer we asked for: TARGETS first, then the text.
func (x *x11) selectionNotify(e xproto.SelectionNotifyEvent) {
	if e.Property == 0 || e.Requestor != x.win {
		return // the owner refused
	}
	r, err := xproto.GetProperty(x.c, true, x.win, e.Property, xproto.AtomAny, 0, 1<<16).Reply()
	if err != nil || r.Type == x.a["INCR"] { // incremental transfers are far beyond what we keep
		return
	}
	if e.Target == x.a["TARGETS"] {
		var names []string
		for _, t := range u32s(r.Value) {
			for n, a := range x.a {
				if uint32(a) == t {
					names = append(names, n)
				}
			}
			if t == uint32(xproto.AtomString) {
				names = append(names, "STRING")
			}
		}
		target := textMime(names)
		if target == "" {
			return
		}
		a := xproto.Atom(xproto.AtomString)
		if target != "STRING" {
			a = x.a[target]
		}
		xproto.ConvertSelection(x.c, x.win, e.Selection, a, x.a["ROUNDBAR_SEL"], e.Time)
		return
	}
	x.mu.Lock()
	var changed bool
	x.clips, changed = pushHistory(x.clips, string(r.Value))
	x.mu.Unlock()
	if changed {
		x.ev.clips()
	}
}

// selectionRequest serves our text to an app pasting it.
func (x *x11) selectionRequest(e xproto.SelectionRequestEvent) {
	x.mu.Lock()
	text := x.own
	x.mu.Unlock()
	prop := e.Property
	if prop == 0 {
		prop = e.Target // obsolete clients
	}
	switch e.Target {
	case x.a["TARGETS"]:
		ts := []xproto.Atom{x.a["TARGETS"], x.a["UTF8_STRING"], x.a["text/plain;charset=utf-8"], x.a["text/plain"], xproto.AtomString, x.a["TEXT"]}
		buf := make([]byte, 0, 4*len(ts))
		for _, t := range ts {
			buf = binary.LittleEndian.AppendUint32(buf, uint32(t))
		}
		xproto.ChangeProperty(x.c, xproto.PropModeReplace, e.Requestor, prop, xproto.AtomAtom, 32, uint32(len(ts)), buf)
	case x.a["UTF8_STRING"], x.a["text/plain;charset=utf-8"], x.a["text/plain"], xproto.AtomString, x.a["TEXT"]:
		typ := e.Target
		if typ == x.a["TEXT"] {
			typ = x.a["UTF8_STRING"]
		}
		xproto.ChangeProperty(x.c, xproto.PropModeReplace, e.Requestor, prop, typ, 8, uint32(len(text)), []byte(text))
	default:
		prop = 0 // refuse
	}
	n := xproto.SelectionNotifyEvent{Time: e.Time, Requestor: e.Requestor, Selection: e.Selection, Target: e.Target, Property: prop}
	xproto.SendEvent(x.c, false, e.Requestor, 0, string(n.Bytes()))
}

func (x *x11) Clipboard() ([]string, error) {
	if !x.xfix {
		return nil, ErrUnsupported
	}
	x.mu.Lock()
	defer x.mu.Unlock()
	return slices.Clone(x.clips), nil
}

func (x *x11) SetClipboard(text string) error {
	if !x.xfix {
		return ErrUnsupported
	}
	x.mu.Lock()
	x.own = text
	t := x.time
	x.clips, _ = pushHistory(x.clips, text)
	x.mu.Unlock()
	if err := xproto.SetSelectionOwnerChecked(x.c, x.win, x.a["CLIPBOARD"], t).Check(); err != nil {
		return err
	}
	r, err := xproto.GetSelectionOwner(x.c, x.a["CLIPBOARD"]).Reply()
	if err != nil || r.Owner != x.win {
		return errors.New("x11: could not take the clipboard")
	}
	x.ev.clips()
	return nil
}
