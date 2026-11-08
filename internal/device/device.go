// Package device finds the roundbar on USB, opens its CDC serial port in raw
// mode and exchanges JSON lines with it (docs/spec.md §2).
package device

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"sync"
	"time"
)

// The board's USB ids (Espressif's VID, our composite CDC + HID product).
const (
	VID uint16 = 0x303a
	PID uint16 = 0x4011
)

// Msg is a decoded line. Raw keeps the original bytes for typed decoding.
type Msg struct {
	T   string          `json:"t"`
	Raw json.RawMessage `json:"-"`
}

type Port struct {
	Path string
	f    conn
	wmu  sync.Mutex
}

// conn is the opened serial port: an *os.File on every platform (Windows
// opens it for overlapped I/O, so deadlines and Close work there too).
type conn interface {
	io.ReadWriteCloser
	SetWriteDeadline(time.Time) error
}

func (p *Port) Close() error { return p.f.Close() }

// Send writes one message as a JSON line.
func (p *Port) Send(v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	b = append(b, '\n')
	p.wmu.Lock()
	defer p.wmu.Unlock()
	_ = p.f.SetWriteDeadline(time.Now().Add(2 * time.Second))
	_, err = p.f.Write(b)
	return err
}

// Read delivers each incoming line to fn until the port fails.
func (p *Port) Read(fn func(Msg)) error {
	sc := bufio.NewScanner(p.f)
	sc.Buffer(make([]byte, 4096), 1<<16)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 || line[0] != '{' {
			continue // boot ROM / log noise
		}
		var m Msg
		if json.Unmarshal(line, &m) != nil || m.T == "" {
			continue
		}
		m.Raw = append(json.RawMessage(nil), line...)
		fn(m)
	}
	if err := sc.Err(); err != nil {
		return err
	}
	return errors.New("device closed")
}

func sleep(done <-chan struct{}, d time.Duration) {
	select {
	case <-done:
	case <-time.After(d):
	}
}
