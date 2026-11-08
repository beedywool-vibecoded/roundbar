package mangohud

import (
	"fmt"
	"math"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseUnix(t *testing.T) {
	raw := []byte(`Num       RefCount Protocol Flags    Type St Inode Path
0000000000000000: 00000002 00000000 00010000 0001 01 101 @mangohud-4242
0000000000000000: 00000003 00000000 00000000 0001 03 102 @mangohud-4242
0000000000000000: 00000002 00000000 00010000 0001 01 103 @other
0000000000000000: 00000002 00000000 00010000 0001 01 104 /run/user/1000/bus
`)
	got := parseUnix(raw, "mangohud-")
	if len(got) != 1 || got[0] != "@mangohud-4242" {
		t.Fatalf("got %v", got)
	}
}

func TestEnsureConf(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	p := ConfPath()
	_ = os.MkdirAll(filepath.Dir(p), 0o755)
	_ = os.WriteFile(p, []byte("fps\nno_display\noutput_folder=/tmp/x"), 0o644)
	if ch, err := EnsureConf(); err != nil || !ch {
		t.Fatalf("first: %v %v", ch, err)
	}
	if ch, _ := EnsureConf(); ch {
		t.Fatal("second call should change nothing")
	}
	raw, _ := os.ReadFile(p)
	if c := readConf(raw); c.control != controlName || c.output != "/tmp/x" || strings.Contains(string(raw), "log_interval") {
		t.Fatalf("conf %+v\n%s", c, raw)
	}
	if bak, _ := os.ReadFile(p + ".roundbar.bak"); string(bak) != "fps\nno_display\noutput_folder=/tmp/x" {
		t.Fatalf("backup %q", bak)
	}
	// the earlier build's log_interval=250 is taken back out
	_ = os.WriteFile(p, []byte("fps\n"+ourComment+"\ncontrol=mangohud-%p\nlog_interval=250\n"), 0o644)
	if ch, _ := EnsureConf(); !ch {
		t.Fatal("should migrate")
	}
	if raw, _ := os.ReadFile(p); string(raw) != "fps\n"+ourComment+"\ncontrol=mangohud-%p\n" {
		t.Fatalf("migrated %q", raw)
	}
	// an existing control name is kept and used
	_ = os.WriteFile(p, []byte("control=mysock\n"), 0o644)
	if ch, _ := EnsureConf(); ch {
		t.Fatal("user settings must be left alone")
	}
	if socketPrefix("mysock") != "mysock" || socketPrefix("") != "mangohud-" {
		t.Fatal("socket prefix")
	}
}

const header = "os,cpu,gpu,ram,kernel,driver,cpuscheduler\nArch,Ryzen,RX,32,6,mesa,\n" +
	"fps,frametime,cpu_load,cpu_power,gpu_load,cpu_temp,gpu_temp,gpu_core_clock,gpu_mem_clock,gpu_vram_used,gpu_power,ram_used,swap_used,process_rss,cpu_mhz,elapsed\n"

func frames(n int, ft float64, gpu int) string {
	var b strings.Builder
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, "%.0f,%.2f,30,40,%d,60,70,2500,1200,6.5,250,12,0,3,4800,1\n", 1000/ft, ft, gpu)
	}
	return b.String()
}

func TestPicksTheRenderingProcess(t *testing.T) {
	dir := t.TempDir()
	s := &Session{dir: dir, before: map[string]bool{}, tails: map[string]*tail{},
		conns: map[string]*ctl{"@mangohud-1": nil}, lastScan: time.Now()}
	old := filepath.Join(dir, "old_2026-01-01_00-00-00.csv")
	_ = os.WriteFile(old, []byte(header+frames(10, 5, 1)), 0o644)
	s.before[old] = true

	// the game at ~160 fps with one 25 ms hitch, and a launcher process at 20 fps
	game := filepath.Join(dir, "game.exe_2026-11-08_20-00-00.csv")
	_ = os.WriteFile(game, []byte(header+frames(300, 6.25, 97)+frames(1, 25, 97)+frames(200, 6.25, 97)), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "launcher.exe_2026-11-08_20-00-00.csv"), []byte(header+frames(40, 50, 3)), 0o644)

	sn := s.Poll()
	if !sn.Active || sn.GPU != 97 {
		t.Fatalf("picked the wrong process: %+v", sn)
	}
	if math.Abs(sn.FPS-160) > 0.5 || math.Abs(sn.Frametime-6.25) > 0.01 {
		t.Fatalf("fps %v ft %v", sn.FPS, sn.Frametime)
	}
	if math.Abs(sn.Low1-100) > 0.01 { // slowest 1% = 5 frames: (25 + 4×6.25) / 5 = 10 ms
		t.Fatalf("1%% low %v", sn.Low1)
	}
	var peak float64
	for _, v := range sn.Graph {
		peak = math.Max(peak, v)
	}
	if peak != 25 {
		t.Fatalf("graph should show the hitch: %v", sn.Graph)
	}

	// rows are appended, partial lines are kept until complete
	fh, _ := os.OpenFile(game, os.O_APPEND|os.O_WRONLY, 0)
	_, _ = fh.WriteString("100,10.00,30,40,9")
	fh.Close()
	before := len(s.tails[game].rows)
	s.Poll()
	if len(s.tails[game].rows) != before {
		t.Fatal("partial row was parsed")
	}
	fh, _ = os.OpenFile(game, os.O_APPEND|os.O_WRONLY, 0)
	_, _ = fh.WriteString("8,60,70,2500,1200,6.5,250,12,0,3,4800,1\n")
	fh.Close()
	if sn := s.Poll(); len(s.tails[game].rows) != before+1 || sn.GPU != 98 {
		t.Fatalf("appended row: %d rows, %+v", len(s.tails[game].rows), sn)
	}
}

func TestWindowTrims(t *testing.T) {
	tl := &tail{}
	dir := t.TempDir()
	f := filepath.Join(dir, "g.csv")
	_ = os.WriteFile(f, []byte(header+frames(4000, 10, 50)), 0o644) // 40 s of frames
	tl.read(f)
	if tl.sumMs > windowMs || tl.sumMs < windowMs-10 || len(tl.rows) != 3000 {
		t.Fatalf("window %v ms, %d rows", tl.sumMs, len(tl.rows))
	}
}

func TestStopRemovesOnlyNewFiles(t *testing.T) {
	dir := t.TempDir()
	keep := filepath.Join(dir, "bench.csv")
	_ = os.WriteFile(keep, nil, 0o644)
	s := &Session{dir: dir, before: map[string]bool{keep: true}, conns: nil}
	_ = os.WriteFile(filepath.Join(dir, "g_2026.csv"), nil, 0o644)
	_ = os.WriteFile(filepath.Join(dir, "g_2026_summary.csv"), nil, 0o644)
	s.Stop()
	left, _ := filepath.Glob(filepath.Join(dir, "*"))
	if len(left) != 1 || left[0] != keep {
		t.Fatalf("left %v", left)
	}
}

// MangoHud serves one client and never accepts another after it: a second page
// visit must reuse the first connection instead of dialing again.
func TestControlConnectionIsReused(t *testing.T) {
	prefix := fmt.Sprintf("rbtest-%d-", os.Getpid())
	l, err := net.Listen("unix", "@"+prefix+"1")
	if err != nil {
		t.Skip("abstract unix sockets unavailable:", err)
	}
	defer l.Close()
	cmds := make(chan string, 16)
	accepts := 0
	go func() {
		c, err := l.Accept() // exactly once, like MangoHud
		if err != nil {
			return
		}
		accepts++
		_, _ = c.Write([]byte(":MangoHudControlVersion=1;"))
		buf := make([]byte, 64)
		var acc string
		for {
			n, err := c.Read(buf)
			if err != nil {
				return
			}
			acc += string(buf[:n])
			for {
				i := strings.IndexByte(acc, ';')
				if i < 0 {
					break
				}
				cmd := acc[:i+1]
				acc = acc[i+1:]
				cmds <- cmd
				if cmd == ":logging=0;" {
					_, _ = c.Write([]byte(":LoggingFinished;"))
				}
			}
		}
	}()
	expect := func(want string) {
		select {
		case got := <-cmds:
			if got != want {
				t.Fatalf("got %q, want %q", got, want)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("no %q", want)
		}
	}
	for visit := 0; visit < 3; visit++ {
		s := &Session{prefix: prefix, dir: t.TempDir(), before: map[string]bool{}, conns: map[string]*ctl{}, tails: map[string]*tail{}}
		s.connect()
		if len(s.conns) != 1 {
			t.Fatalf("visit %d: not connected", visit)
		}
		expect(":logging=1;")
		s.Stop()
		expect(":logging=0;")
	}
	if accepts != 1 {
		t.Fatalf("dialed %d times", accepts)
	}
}
