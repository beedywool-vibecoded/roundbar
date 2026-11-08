//go:build linux

package desktop

import (
	"encoding/binary"
	"errors"
	"net"
	"os"
	"path/filepath"
	"sync"

	"golang.org/x/sys/unix"
)

// A minimal Wayland client: the wire format, object ids and fd passing, just
// enough for the few protocols the wayland backend speaks. One goroutine
// blocks reading the socket and dispatches events; requests may come from any
// goroutine.

type wlConn struct {
	c *net.UnixConn

	wmu  sync.Mutex // serializes writes and id allocation (ids must be sequential)
	next uint32

	hmu      sync.Mutex
	handlers map[uint32]wlHandler

	fds []int // received fds, consumed in order by events that carry one
}

type wlHandler func(op uint16, m *wlDec)

func wlDial() (*wlConn, error) {
	names := []string{os.Getenv("WAYLAND_DISPLAY")}
	if names[0] == "" { // not in the service's environment: try the usual sockets
		names = []string{"wayland-0", "wayland-1", "wayland-2"}
	}
	var err error
	for _, name := range names {
		if !filepath.IsAbs(name) {
			name = filepath.Join(runtimeDir(), name)
		}
		var c *net.UnixConn
		if c, err = net.DialUnix("unix", nil, &net.UnixAddr{Name: name, Net: "unix"}); err == nil {
			return &wlConn{c: c, next: 2, handlers: map[uint32]wlHandler{}}, nil // 1 is wl_display
		}
	}
	return nil, err
}

func (w *wlConn) Close() { w.c.Close() }

// on sets (or with nil, drops) the event handler for an object.
func (w *wlConn) on(id uint32, h wlHandler) {
	w.hmu.Lock()
	if h == nil {
		delete(w.handlers, id)
	} else {
		w.handlers[id] = h
	}
	w.hmu.Unlock()
}

// wlArgs builds a request body.
type wlArgs []byte

func (a wlArgs) u(v uint32) wlArgs { return binary.LittleEndian.AppendUint32(a, v) }

func (a wlArgs) s(v string) wlArgs {
	a = a.u(uint32(len(v) + 1))
	a = append(a, v...)
	a = append(a, 0)
	for len(a)%4 != 0 {
		a = append(a, 0)
	}
	return a
}

// req sends a request. newID != nil allocates an id, fills it in place of
// the -1 placeholder arg it returns, and registers h for it; fd >= 0 is passed along.
func (w *wlConn) req(obj uint32, op uint16, build func(newID uint32) wlArgs, h wlHandler, fd int) (uint32, error) {
	w.wmu.Lock()
	defer w.wmu.Unlock()
	id := uint32(0)
	if h != nil {
		id = w.next
		w.next++
		w.on(id, h)
	}
	var body wlArgs
	if build != nil {
		body = build(id)
	}
	msg := wlArgs{}.u(obj).u(uint32(8+len(body))<<16 | uint32(op))
	msg = append(msg, body...)
	var oob []byte
	if fd >= 0 {
		oob = unix.UnixRights(fd)
	}
	_, _, err := w.c.WriteMsgUnix(msg, oob, nil)
	return id, err
}

// wlDec reads event arguments; a short message leaves zero values.
type wlDec struct {
	b  []byte
	fd func() int
}

func (d *wlDec) u() uint32 {
	if len(d.b) < 4 {
		d.b = nil
		return 0
	}
	v := binary.LittleEndian.Uint32(d.b)
	d.b = d.b[4:]
	return v
}

func (d *wlDec) arr() []byte {
	n := int(d.u())
	p := (n + 3) &^ 3
	if n < 0 || p > len(d.b) {
		d.b = nil
		return nil
	}
	v := d.b[:n]
	d.b = d.b[p:]
	return v
}

func (d *wlDec) s() string {
	v := d.arr()
	if len(v) > 0 && v[len(v)-1] == 0 {
		v = v[:len(v)-1]
	}
	return string(v)
}

var errWlClosed = errors.New("wayland: connection closed")

// loop reads and dispatches events until the connection closes.
func (w *wlConn) loop() error {
	buf := make([]byte, 0, 8192)
	tmp := make([]byte, 8192)
	oob := make([]byte, unix.CmsgSpace(28*4))
	for {
		n, oobn, _, _, err := w.c.ReadMsgUnix(tmp, oob)
		if oobn > 0 {
			if cms, err := unix.ParseSocketControlMessage(oob[:oobn]); err == nil {
				for _, cm := range cms {
					if fds, err := unix.ParseUnixRights(&cm); err == nil {
						w.fds = append(w.fds, fds...)
					}
				}
			}
		}
		if err != nil {
			return err
		}
		if n == 0 {
			return errWlClosed
		}
		buf = append(buf, tmp[:n]...)
		buf = w.dispatch(buf)
	}
}

// dispatch handles every complete message in buf and returns the rest.
func (w *wlConn) dispatch(all []byte) []byte {
	buf := all
	for len(buf) >= 8 {
		obj := binary.LittleEndian.Uint32(buf)
		hdr := binary.LittleEndian.Uint32(buf[4:])
		size := int(hdr >> 16)
		if size < 8 {
			return nil // garbage: drop it
		}
		if len(buf) < size {
			break
		}
		w.hmu.Lock()
		h := w.handlers[obj]
		w.hmu.Unlock()
		if h != nil {
			h(uint16(hdr), &wlDec{b: buf[8:size], fd: w.popFD})
		}
		buf = buf[size:]
	}
	return all[:copy(all, buf)]
}

func (w *wlConn) popFD() int {
	if len(w.fds) == 0 {
		return -1
	}
	fd := w.fds[0]
	w.fds = w.fds[1:]
	return fd
}
