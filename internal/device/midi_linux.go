package device

import (
	"bytes"
	"os"
	"path/filepath"
	"time"
)

// FindMIDI returns the raw MIDI device (/dev/snd/midiC1D0) of an attached
// roundbar, or "". Sound devices are open to the logged-in user without any
// udev rule of ours, so this is the way in when the serial port isn't.
func FindMIDI() string {
	nodes, _ := filepath.Glob("/sys/class/sound/midiC*D*")
	for _, n := range nodes {
		dev, err := filepath.EvalSymlinks(filepath.Join(n, "device"))
		if err != nil {
			continue
		}
		for d := dev; d != "/" && d != "."; d = filepath.Dir(d) {
			v, err1 := os.ReadFile(filepath.Join(d, "idVendor"))
			p, err2 := os.ReadFile(filepath.Join(d, "idProduct"))
			if err1 != nil || err2 != nil {
				continue
			}
			if hexID(v) == VID && hexID(p) == PID {
				return "/dev/snd/" + filepath.Base(n)
			}
			break
		}
	}
	return ""
}

// OpenMIDI opens the board's raw MIDI device and speaks JSON lines over SysEx.
func OpenMIDI(path string) (*Port, error) {
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return nil, err
	}
	return &Port{Path: path, f: &midiConn{f: f}}, nil
}

// midiConn presents a raw MIDI device as a stream of newline-ended lines.
type midiConn struct {
	f       *os.File
	dec     sysexDecoder
	pending []byte
	buf     [4096]byte
}

func (c *midiConn) Read(p []byte) (int, error) {
	for len(c.pending) == 0 {
		n, err := c.f.Read(c.buf[:])
		c.dec.feed(c.buf[:n], func(line []byte) {
			c.pending = append(c.pending, line...)
			c.pending = append(c.pending, '\n')
		})
		if err != nil && len(c.pending) == 0 {
			return 0, err
		}
	}
	n := copy(p, c.pending)
	c.pending = c.pending[n:]
	return n, nil
}

// Write takes whole lines (Port.Send writes exactly one) and sends each as SysEx.
func (c *midiConn) Write(p []byte) (int, error) {
	for _, line := range bytes.Split(bytes.TrimSuffix(p, []byte{'\n'}), []byte{'\n'}) {
		if _, err := c.f.Write(encodeSysEx(line)); err != nil {
			return 0, err
		}
	}
	return len(p), nil
}

func (c *midiConn) SetWriteDeadline(t time.Time) error { return c.f.SetWriteDeadline(t) }
func (c *midiConn) Close() error                       { return c.f.Close() }
