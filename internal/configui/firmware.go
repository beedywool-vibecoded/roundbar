package configui

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/beedywool-vibecoded/roundbar/internal/fwupdate"
	"github.com/beedywool-vibecoded/roundbar/internal/ipc"
)

// RepoDir is the roundbar checkout this binary was built from (set by the
// Makefile with -ldflags -X). Developers can build and flash from there; the
// normal path installs published releases (internal/fwupdate).
var RepoDir string

func idfExport() string {
	if p := os.Getenv("IDF_PATH"); p != "" {
		if _, err := os.Stat(filepath.Join(p, "export.sh")); err == nil {
			return filepath.Join(p, "export.sh")
		}
	}
	home, _ := os.UserHomeDir()
	m, _ := filepath.Glob(filepath.Join(home, "esp", "esp-idf-*", "export.sh"))
	sort.Strings(m)
	if len(m) > 0 {
		return m[len(m)-1]
	}
	return ""
}

func builtVersion() string {
	if RepoDir == "" {
		return ""
	}
	raw, err := os.ReadFile(filepath.Join(RepoDir, "firmware", "build", "project_description.json"))
	if err != nil {
		return ""
	}
	var d struct {
		Version string `json:"project_version"`
	}
	_ = json.Unmarshal(raw, &d)
	return d.Version
}

type flashJob struct {
	mu      sync.Mutex
	running bool
}

var job flashJob

func (j *flashJob) isRunning() bool {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.running
}

// start claims the job; false if one is already running.
func (j *flashJob) start() bool {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.running {
		return false
	}
	j.running = true
	return true
}

func (j *flashJob) finish() {
	j.mu.Lock()
	j.running = false
	j.mu.Unlock()
}

func (s *server) getFirmware(w http.ResponseWriter, r *http.Request) {
	repoOK := false
	if RepoDir != "" {
		_, err := os.Stat(filepath.Join(RepoDir, "tools", "flash.sh"))
		repoOK = err == nil
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"repo": RepoDir, "repoOk": repoOK, "idf": idfExport(), "built": builtVersion(), "running": job.isRunning(),
	})
}

// The latest release, fetched only while the configurator is open: when the
// Firmware card asks (GET /api/firmware/release), at most every 10 minutes
// unless ?refresh=1. Nothing polls in the background.
type releaseCache struct {
	mu   sync.Mutex
	m    *fwupdate.Manifest
	err  error
	when time.Time
}

var latest releaseCache

func (c *releaseCache) get(ctx context.Context, refresh bool) (*fwupdate.Manifest, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !refresh && !c.when.IsZero() && time.Since(c.when) < 10*time.Minute {
		return c.m, c.err
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	c.m, c.err = fwupdate.NewFetcher().Latest(ctx)
	c.when = time.Now()
	return c.m, c.err
}

func deviceFW() (fw string, connected bool) {
	var st struct {
		Device struct {
			Connected bool   `json:"connected"`
			FW        string `json:"fw"`
		} `json:"device"`
	}
	if ipc.Call(ipc.Request{Op: "status"}, &st) != nil {
		return "", false
	}
	return st.Device.FW, st.Device.Connected
}

// getRelease: {version, status, size, fontsSize} or {error}.
func (s *server) getRelease(w http.ResponseWriter, r *http.Request) {
	m, err := latest.get(r.Context(), r.URL.Query().Get("refresh") == "1")
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		_ = json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
		return
	}
	fw, connected := deviceFW()
	status := fwupdate.Unknown
	if connected {
		status = fwupdate.Compare(fw, m.Version)
	}
	app, _ := m.Part("app")
	fonts, _ := m.Part("fonts")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"version": m.Version, "status": status, "appSize": app.Size, "fontsSize": fonts.Size,
	})
}

// postFirmware starts an update: {"source":"release"} (default) installs the
// latest release, {"source":"build"} builds and flashes this PC's checkout.
// Progress goes to every SSE client as `event: flash`: {line}, {progress:
// {done,total}} while writing, then {done:true, ok}.
func (s *server) postFirmware(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Source string `json:"source"`
	}
	_ = json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&body)
	build := body.Source == "build"
	idf := idfExport()
	if build && (RepoDir == "" || idf == "") {
		jsonErr(w, http.StatusPreconditionFailed, "needs the roundbar checkout and ESP-IDF (~/esp/esp-idf-*)", "")
		return
	}
	var m *fwupdate.Manifest
	if !build {
		var err error
		if m, err = latest.get(r.Context(), false); err != nil {
			jsonErr(w, http.StatusBadGateway, "couldn't get the latest release: "+err.Error(), "")
			return
		}
	}
	if !job.start() {
		jsonErr(w, http.StatusConflict, "an update is already running", "")
		return
	}
	if build {
		if err := s.runBuild(idf); err != nil {
			job.finish()
			jsonErr(w, http.StatusInternalServerError, err.Error(), "")
			return
		}
	} else {
		go s.runRelease(m)
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"ok":true}`))
}

func (s *server) flashLine(line string) {
	b, _ := json.Marshal(map[string]any{"line": line})
	s.broadcastEvent("flash", b)
}

func (s *server) flashDone(ok bool) {
	b, _ := json.Marshal(map[string]any{"done": true, "ok": ok})
	s.broadcastEvent("flash", b)
	job.finish()
}

func (s *server) runRelease(m *fwupdate.Manifest) {
	f := fwupdate.NewFetcher()
	lastPct := -1
	progress := func(done, total int) {
		if total <= 0 {
			return
		}
		if pct := done * 100 / total; pct != lastPct {
			lastPct = pct
			b, _ := json.Marshal(map[string]any{"progress": map[string]int{"done": done, "total": total}})
			s.broadcastEvent("flash", b)
		}
	}
	f.Progress = func(file string, done, total int64) { progress(int(done), int(total)) }
	u := fwupdate.New(m, f, func() error { return ipc.Call(ipc.Request{Op: "bootloader"}, nil) })
	u.Log = s.flashLine
	u.Progress = progress
	s.flashLine("installing " + m.Version)
	err := u.Run(context.Background())
	if err != nil {
		s.flashLine("error: " + err.Error())
	} else {
		f.Prune(m)
		s.flashLine("done: the device is restarting with " + m.Version)
	}
	s.flashDone(err == nil)
}

// runBuild is the developer path: build the checkout with ESP-IDF and flash
// it with tools/flash.sh (esptool).
func (s *server) runBuild(idf string) error {
	script := `. "$1" >/dev/null && cd "$2/firmware" && idf.py build && "$2/tools/flash.sh"`
	cmd := exec.Command("bash", "-c", script, "bash", idf, RepoDir)
	out, _ := cmd.StdoutPipe()
	cmd.Stderr = cmd.Stdout
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() {
		sc := bufio.NewScanner(out)
		sc.Buffer(make([]byte, 4096), 1<<20)
		for sc.Scan() {
			line := strings.TrimRight(sc.Text(), "\r")
			if i := strings.LastIndex(line, "\r"); i >= 0 { // esptool progress bars
				line = line[i+1:]
			}
			if line != "" {
				s.flashLine(line)
			}
		}
		s.flashDone(cmd.Wait() == nil)
	}()
	return nil
}
