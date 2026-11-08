// Package mangohud reads live frame metrics from MangoHud while a MangoHud
// page is on the device. MangoHud has no live API: its control socket can only
// toggle things, so we turn its CSV logging on over that socket, tail the file
// it writes, and turn logging off (and delete the files we caused) when the
// page goes away. Nothing shows in the game: logging is independent of the HUD.
package mangohud

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"math"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Settings Roundbar needs in MangoHud.conf. `%p` gives one socket per game.
//
// Logging stays per frame (MangoHud's default log_interval=0): only a process
// that presents frames writes rows, and the stats are real frame times. With a
// time interval, idle helper processes of a game (Wine launchers) keep writing
// their last values, which made the page jump around.
const (
	controlName = "mangohud-%p"
	ourComment  = "# Roundbar: live stats for its MangoHud page (logging only while the page is shown)"
	windowMs    = 30000.0 // stats window, in summed frame time
	graphLen    = 60
	bucketMs    = 250.0 // one graph point: the slowest frame in 250 ms
)

func ConfPath() string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "MangoHud", "MangoHud.conf")
}

type conf struct {
	output, control, toggle string
}

func readConf(raw []byte) conf {
	var c conf
	for _, line := range strings.Split(string(raw), "\n") {
		k, v, _ := strings.Cut(strings.TrimSpace(line), "=")
		switch strings.TrimSpace(k) {
		case "output_folder":
			c.output = strings.TrimSpace(v)
		case "control":
			c.control = strings.TrimSpace(v)
		case "toggle_logging":
			c.toggle = strings.TrimSpace(v)
		}
	}
	return c
}

// EnsureConf adds the control socket to MangoHud.conf when it's missing,
// keeping a one-time backup. Games started before the change need a restart
// (MangoHud reads its config at launch).
func EnsureConf() (changed bool, err error) {
	p := ConfPath()
	raw, err := os.ReadFile(p)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	// an earlier Roundbar build also set log_interval=250: take it back out
	if old := []byte("control=" + controlName + "\nlog_interval=250\n"); bytes.Contains(raw, old) {
		raw = bytes.Replace(raw, old, []byte("control="+controlName+"\n"), 1)
		return true, os.WriteFile(p, raw, 0o644)
	}
	c := readConf(raw)
	var add []string
	if c.control == "" {
		add = append(add, "control="+controlName)
	}
	if len(add) == 0 {
		return false, nil
	}
	if len(raw) > 0 {
		bak := p + ".roundbar.bak"
		if _, err := os.Stat(bak); errors.Is(err, os.ErrNotExist) {
			_ = os.WriteFile(bak, raw, 0o644)
		}
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return false, err
	}
	if len(raw) > 0 && !bytes.HasSuffix(raw, []byte("\n")) {
		raw = append(raw, '\n')
	}
	raw = append(raw, []byte(ourComment+"\n"+strings.Join(add, "\n")+"\n")...)
	return true, os.WriteFile(p, raw, 0o644)
}

// socketPrefix is the abstract socket name before the PID.
func socketPrefix(control string) string {
	if control == "" {
		control = controlName
	}
	pre, _, _ := strings.Cut(control, "%p")
	return pre
}

// Sockets lists MangoHud control sockets (abstract unix names) from /proc/net/unix.
func Sockets(prefix string) []string {
	raw, err := os.ReadFile("/proc/net/unix")
	if err != nil {
		return nil
	}
	return parseUnix(raw, prefix)
}

func parseUnix(raw []byte, prefix string) []string {
	seen := map[string]bool{}
	var out []string
	for _, line := range strings.Split(string(raw), "\n") {
		f := strings.Fields(line)
		if len(f) < 8 {
			continue
		}
		// listening sockets only (state 01 = unconnected listening, flags 00010000 = __SO_ACCEPTCON)
		name := f[len(f)-1]
		if strings.HasPrefix(name, "@"+prefix) && f[3] == "00010000" && !seen[name] {
			seen[name] = true
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

// Snapshot is what the device page shows.
type Snapshot struct {
	Active    bool      `json:"active"`
	Hint      string    `json:"hint,omitempty"` // why there's no data
	FPS       float64   `json:"fps"`
	Frametime float64   `json:"ft"`
	Avg       float64   `json:"avg"`
	Low1      float64   `json:"low"`
	CPU       float64   `json:"cpu"`
	GPU       float64   `json:"gpu"`
	CPUTemp   float64   `json:"cpuTemp"`
	GPUTemp   float64   `json:"gpuTemp"`
	Power     float64   `json:"power"`
	VRAM      float64   `json:"vram"`
	Graph     []float64 `json:"graph"` // recent frametimes, ms
}

type row struct {
	ft, cpu, gpu, cpuTemp, gpuTemp, power, vram float64
}

// tail follows one CSV (one game process).
type tail struct {
	off     int64
	partial []byte
	cols    map[string]int
	rows    []row
	sumMs   float64 // frame time covered by rows
	lastNew time.Time
}

// Session logs while a MangoHud page is visible.
type Session struct {
	prefix, dir string
	before      map[string]bool // files in the log folder before we started
	conns       map[string]*ctl
	keys        Keys
	combo       string // MangoHud's toggle_logging in board syntax
	started     time.Time
	keyOn       bool // we pressed the shortcut to start logging
	tails       map[string]*tail
	lastScan    time.Time
}

func logDir(c conf) string {
	if c.output != "" {
		if strings.HasPrefix(c.output, "~/") {
			home, _ := os.UserHomeDir()
			return filepath.Join(home, c.output[2:])
		}
		return c.output
	}
	home, _ := os.UserHomeDir()
	return home
}

// Keys lets the session press MangoHud's logging shortcut through the board
// (a USB keyboard) when the control socket doesn't reach the game: some games
// (several graphics instances, e.g. DX12 + anti-cheat) never service it, while
// MangoHud's keybinds always reach the instance that draws.
type Keys struct {
	Press   func(keys string) // types a key combo, e.g. "shift+f2"
	Focused func() bool       // the game still has the focus
}

// toggleKeys converts MangoHud's toggle_logging (XKB names joined by +, default
// Shift_L+F2) to the board's key syntax; "" when it can't be typed.
func toggleKeys(v string) string {
	if v == "" {
		v = "Shift_L+F2"
	}
	var out []string
	for _, k := range strings.Split(v, "+") {
		k = strings.TrimSpace(k)
		switch strings.ToLower(strings.TrimSuffix(strings.TrimSuffix(k, "_L"), "_R")) {
		case "shift":
			out = append(out, "shift")
		case "control", "ctrl":
			out = append(out, "ctrl")
		case "alt", "meta":
			out = append(out, "alt")
		case "super":
			out = append(out, "super")
		default:
			if len(k) == 0 {
				return ""
			}
			out = append(out, strings.ToLower(k))
		}
	}
	return strings.Join(out, "+")
}

func Start(keys Keys) *Session {
	raw, _ := os.ReadFile(ConfPath())
	c := readConf(raw)
	s := &Session{prefix: socketPrefix(c.control), dir: logDir(c), before: map[string]bool{}, conns: map[string]*ctl{},
		tails: map[string]*tail{}, keys: keys, combo: toggleKeys(c.toggle), started: time.Now()}
	for _, f := range s.csvs() {
		// a log still being written (left running by an earlier visit) is adopted, not ignored
		if st, err := os.Stat(f); err == nil && time.Since(st.ModTime()) > 3*time.Second {
			s.before[f] = true
		}
	}
	s.connect()
	return s
}

func (s *Session) csvs() []string {
	m, _ := filepath.Glob(filepath.Join(s.dir, "*.csv"))
	return m
}

// connect starts logging on games that appeared since the last scan.
// ctl is a control connection to one game's MangoHud, kept for the game's whole
// life: MangoHud serves a single client and doesn't take a new one after the
// first disconnects, so closing it would lock us out until the game restarts.
type ctl struct {
	c        net.Conn
	finished chan struct{} // one value per "LoggingFinished"
	dead     chan struct{} // closed when the game (or its socket) goes away
}

var pool = struct {
	sync.Mutex
	m map[string]*ctl
}{m: map[string]*ctl{}}

// control returns the live connection to a MangoHud socket, dialing it once.
func control(name string) *ctl {
	pool.Lock()
	defer pool.Unlock()
	if k := pool.m[name]; k != nil {
		select {
		case <-k.dead:
			delete(pool.m, name)
		default:
			return k
		}
	}
	c, err := net.DialTimeout("unix", name, time.Second)
	if err != nil {
		return nil
	}
	k := &ctl{c: c, finished: make(chan struct{}, 4), dead: make(chan struct{})}
	go func() { // version strings, then LoggingFinished each time logging stops
		defer func() {
			close(k.dead)
			c.Close()
			pool.Lock()
			if pool.m[name] == k {
				delete(pool.m, name)
			}
			pool.Unlock()
		}()
		buf := make([]byte, 512)
		var seen []byte
		for {
			n, err := c.Read(buf)
			seen = append(seen, buf[:n]...)
			for bytes.Contains(seen, []byte("LoggingFinished")) {
				_, after, _ := bytes.Cut(seen, []byte("LoggingFinished"))
				seen = append([]byte(nil), after...)
				select {
				case k.finished <- struct{}{}:
				default:
				}
			}
			if len(seen) > 4096 {
				seen = seen[len(seen)-64:]
			}
			if err != nil {
				return
			}
		}
	}()
	pool.m[name] = k
	return k
}

// connect starts logging on games that appeared since the last scan.
func (s *Session) connect() {
	s.lastScan = time.Now()
	for _, name := range Sockets(s.prefix) {
		if s.conns[name] != nil {
			continue
		}
		k := control(name)
		if k == nil {
			continue
		}
		for len(k.finished) > 0 { // stale signals from an earlier visit
			<-k.finished
		}
		if _, err := k.c.Write([]byte(":logging=1;")); err != nil {
			k.c.Close() // the reader sees it and drops the connection
			continue
		}
		s.conns[name] = k
	}
}

// Poll reads what MangoHud logged since the last call and summarizes the
// process that is actually rendering: a game can run several MangoHud
// processes (Wine launchers and helpers), each logging to its own file.
func (s *Session) Poll() Snapshot {
	if time.Since(s.lastScan) > 2*time.Second {
		s.connect()
	}
	for _, f := range s.csvs() {
		if s.before[f] || strings.HasSuffix(f, "_summary.csv") {
			continue
		}
		t := s.tails[f]
		if t == nil {
			t = &tail{}
			s.tails[f] = t
		}
		t.read(f)
	}
	var best *tail
	for _, t := range s.tails {
		if time.Since(t.lastNew) > 3*time.Second || len(t.rows) == 0 {
			continue
		}
		if best == nil || t.fpsNow() > best.fpsNow() {
			best = t
		}
	}
	if best == nil {
		// Nothing logged yet: if the socket didn't do it, press the shortcut once.
		if !s.keyOn && len(s.tails) == 0 && s.combo != "" && s.keys.Press != nil && time.Since(s.started) > 1500*time.Millisecond &&
			(s.keys.Focused == nil || s.keys.Focused()) {
			s.keys.Press(s.combo)
			s.keyOn = true
		}
		if len(s.conns) == 0 && !s.keyOn {
			return Snapshot{Hint: "noGame"}
		}
		return Snapshot{Hint: "waiting"}
	}
	return summarize(best.rows)
}

func (t *tail) read(file string) {
	f, err := os.Open(file)
	if err != nil {
		return
	}
	defer f.Close()
	if _, err := f.Seek(t.off, io.SeekStart); err != nil {
		return
	}
	data, _ := io.ReadAll(io.LimitReader(f, 4<<20))
	t.off += int64(len(data))
	data = append(t.partial, data...)
	end := bytes.LastIndexByte(data, '\n')
	if end < 0 {
		t.partial = data
		return
	}
	t.partial = append([]byte(nil), data[end+1:]...)
	sc := bufio.NewScanner(bytes.NewReader(data[:end]))
	for sc.Scan() {
		if r, ok := t.parse(sc.Text()); ok && r.ft > 0 {
			t.rows = append(t.rows, r)
			t.sumMs += r.ft
			t.lastNew = time.Now()
		}
	}
	drop := 0
	for t.sumMs > windowMs && drop < len(t.rows)-1 {
		t.sumMs -= t.rows[drop].ft
		drop++
	}
	if drop > 0 {
		t.rows = append(t.rows[:0:0], t.rows[drop:]...)
	}
}

func (t *tail) parse(line string) (row, bool) {
	f := strings.Split(strings.TrimSpace(line), ",")
	if len(f) > 1 && f[0] == "fps" && f[1] == "frametime" {
		t.cols = map[string]int{}
		for i, name := range f {
			t.cols[name] = i
		}
		return row{}, false
	}
	if t.cols == nil || len(f) < len(t.cols)-1 {
		return row{}, false
	}
	get := func(name string) float64 {
		i, ok := t.cols[name]
		if !ok || i >= len(f) {
			return 0
		}
		v, _ := strconv.ParseFloat(f[i], 64)
		return v
	}
	return row{ft: get("frametime"), cpu: get("cpu_load"), gpu: get("gpu_load"),
		cpuTemp: get("cpu_temp"), gpuTemp: get("gpu_temp"), power: get("gpu_power"), vram: get("gpu_vram_used")}, true
}

func (t *tail) fpsNow() float64 { return recentFPS(t.rows, 1000) }

// recentFPS counts the frames in the last spanMs of frame time.
func recentFPS(rows []row, spanMs float64) float64 {
	var sum float64
	n := 0
	for i := len(rows) - 1; i >= 0 && sum < spanMs; i-- {
		sum += rows[i].ft
		n++
	}
	if sum == 0 {
		return 0
	}
	return float64(n) * 1000 / sum
}

func summarize(rows []row) Snapshot {
	last := rows[len(rows)-1]
	sn := Snapshot{Active: true, FPS: recentFPS(rows, 1000), CPU: last.cpu, GPU: last.gpu,
		CPUTemp: last.cpuTemp, GPUTemp: last.gpuTemp, Power: last.power, VRAM: last.vram}
	sn.Frametime = 1000 / sn.FPS // average over the last second, steadier than one frame
	fts := make([]float64, len(rows))
	var sum float64
	for i, r := range rows {
		fts[i] = r.ft
		sum += r.ft
	}
	sn.Avg = float64(len(rows)) * 1000 / sum
	// 1% low: average of the slowest 1% of frames (at least one)
	sorted := append([]float64(nil), fts...)
	sort.Float64s(sorted)
	k := max(1, len(sorted)/100)
	var worst float64
	for _, v := range sorted[len(sorted)-k:] {
		worst += v
	}
	sn.Low1 = 1000 / (worst / float64(k))
	// graph, oldest first: the slowest frame of each 250 ms, so stutters show
	var bucket, peak float64
	for i := len(fts) - 1; i >= 0 && len(sn.Graph) < graphLen; i-- {
		peak = math.Max(peak, fts[i])
		bucket += fts[i]
		if bucket >= bucketMs || i == 0 {
			sn.Graph = append(sn.Graph, math.Round(peak*10)/10)
			bucket, peak = 0, 0
		}
	}
	for i, j := 0, len(sn.Graph)-1; i < j; i, j = i+1, j-1 {
		sn.Graph[i], sn.Graph[j] = sn.Graph[j], sn.Graph[i]
	}
	return sn
}

// Stop turns logging off and removes the CSV and summary files it created.
func (s *Session) Stop() {
	for _, k := range s.conns {
		_, _ = k.c.Write([]byte(":logging=0;"))
	}
	// MangoHud writes the summary, then says LoggingFinished. The connections
	// stay open for the next visit (see ctl).
	deadline := time.After(3 * time.Second)
wait:
	for _, k := range s.conns {
		select {
		case <-k.finished:
		case <-k.dead:
		case <-deadline:
			break wait
		}
	}
	// Logging we started with the shortcut: press it again if it's still running,
	// but only while the game has the focus (never type it into another app).
	if s.keyOn && s.growing() && (s.keys.Focused == nil || s.keys.Focused()) {
		s.keys.Press(s.combo)
		time.Sleep(time.Second) // MangoHud writes the summary
	}
	for _, f := range s.csvs() {
		if !s.before[f] && !growingFile(f) { // a log left running is reused next visit
			_ = os.Remove(f)
		}
	}
}

func growingFile(f string) bool {
	a, err := os.Stat(f)
	if err != nil {
		return false
	}
	time.Sleep(400 * time.Millisecond)
	b, err := os.Stat(f)
	return err == nil && b.Size() > a.Size()
}

// growing reports whether a log of this session is still being written.
func (s *Session) growing() bool {
	for f := range s.tails {
		if growingFile(f) {
			return true
		}
	}
	return false
}
