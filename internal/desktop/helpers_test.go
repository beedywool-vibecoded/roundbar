//go:build linux

package desktop

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// recorder collects backend events.
type recorder struct {
	focusC chan Focus
	winC   chan struct{}
	clipC  chan struct{}
	notifC chan string
}

func newRecorder() *recorder {
	return &recorder{make(chan Focus, 16), make(chan struct{}, 64), make(chan struct{}, 16), make(chan string, 16)}
}

func (r *recorder) focus(f Focus) { r.focusC <- f }
func (r *recorder) windows() {
	select {
	case r.winC <- struct{}{}:
	default:
	}
}
func (r *recorder) clips()          { r.clipC <- struct{}{} }
func (r *recorder) notif(id string) { r.notifC <- id }

func wait[T any](t *testing.T, ch chan T, what string) T {
	t.Helper()
	select {
	case v := <-ch:
		return v
	case <-time.After(3 * time.Second):
		t.Fatalf("timed out waiting for %s", what)
	}
	var zero T
	return zero
}

// testApps points the desktop entry index at a temp dir holding these entries.
func testApps(t *testing.T, entries map[string]string) {
	t.Helper()
	dir := t.TempDir()
	apps := filepath.Join(dir, "applications")
	if err := os.MkdirAll(apps, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range entries {
		if err := os.WriteFile(filepath.Join(apps, name+".desktop"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("XDG_DATA_HOME", dir)
	t.Setenv("XDG_DATA_DIRS", filepath.Join(dir, "none"))
	t.Setenv("HOME", dir)
	resetApps()
	t.Cleanup(resetApps)
}

func resetApps() {
	apps.Lock()
	apps.strong, apps.weak = nil, nil
	apps.Unlock()
}

// privateBus starts a throwaway session bus and points the process at it.
func privateBus(t *testing.T) {
	t.Helper()
	bin, err := exec.LookPath("dbus-daemon")
	if err != nil {
		t.Skip("dbus-daemon not installed")
	}
	cmd := exec.Command(bin, "--session", "--nofork", "--print-address=1")
	out, _ := cmd.StdoutPipe()
	if err := cmd.Start(); err != nil {
		t.Skip(err)
	}
	t.Cleanup(func() { cmd.Process.Kill(); cmd.Wait() })
	addr, err := bufio.NewReader(out).ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", strings.TrimSpace(addr))
}

func waitShort() <-chan time.Time { return time.After(100 * time.Millisecond) }
