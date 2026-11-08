// Package ipc is the daemon's unix socket protocol (docs/spec.md §3): one JSON
// request per line, one JSON reply per line. "watch" streams status lines.
package ipc

import (
	"bufio"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"time"
)

func SocketPath() string {
	dir := os.Getenv("XDG_RUNTIME_DIR")
	if dir == "" {
		dir = filepath.Join(os.TempDir(), "roundbar-"+os.Getenv("USER"))
		_ = os.MkdirAll(dir, 0o700)
	}
	return filepath.Join(dir, "roundbar.sock")
}

type Request struct {
	Op      string          `json:"op"`
	Profile string          `json:"profile,omitempty"`
	Page    string          `json:"page,omitempty"`
	Action  json.RawMessage `json:"action,omitempty"`
}

// Call sends one request and decodes one reply into out.
func Call(req Request, out any) error {
	c, err := net.DialTimeout("unix", SocketPath(), time.Second)
	if err != nil {
		return err
	}
	defer c.Close()
	_ = c.SetDeadline(time.Now().Add(15 * time.Second))
	if err := json.NewEncoder(c).Encode(req); err != nil {
		return err
	}
	line, err := bufio.NewReader(c).ReadBytes('\n')
	if err != nil {
		return err
	}
	var e struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(line, &e) == nil && e.Error != "" {
		return errors.New(e.Error)
	}
	if out != nil {
		return json.Unmarshal(line, out)
	}
	return nil
}

// Watch streams raw status lines to fn until the connection drops or fn returns false.
func Watch(fn func(json.RawMessage) bool) error {
	c, err := net.DialTimeout("unix", SocketPath(), time.Second)
	if err != nil {
		return err
	}
	defer c.Close()
	if err := json.NewEncoder(c).Encode(Request{Op: "watch"}); err != nil {
		return err
	}
	r := bufio.NewReader(c)
	for {
		line, err := r.ReadBytes('\n')
		if err != nil {
			return err
		}
		if !fn(json.RawMessage(line)) {
			return nil
		}
	}
}
