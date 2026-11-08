package configui

import (
	"bufio"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// RepoDir is the roundbar checkout this binary was built from (set by the
// Makefile with -ldflags -X). Firmware updates build and flash from there.
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

func (s *server) getFirmware(w http.ResponseWriter, r *http.Request) {
	repoOK := false
	if RepoDir != "" {
		_, err := os.Stat(filepath.Join(RepoDir, "tools", "flash.sh"))
		repoOK = err == nil
	}
	job.mu.Lock()
	running := job.running
	job.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"repo": RepoDir, "repoOk": repoOK, "idf": idfExport(), "built": builtVersion(), "running": running,
	})
}

// postFirmware builds the firmware and flashes it; output lines go to every
// SSE client as `event: flash`, ending with {"done":true,"ok":…}.
func (s *server) postFirmware(w http.ResponseWriter, r *http.Request) {
	idf := idfExport()
	if RepoDir == "" || idf == "" {
		jsonErr(w, http.StatusPreconditionFailed, "needs the roundbar checkout and ESP-IDF (~/esp/esp-idf-*)", "")
		return
	}
	job.mu.Lock()
	if job.running {
		job.mu.Unlock()
		jsonErr(w, http.StatusConflict, "an update is already running", "")
		return
	}
	job.running = true
	job.mu.Unlock()

	script := `. "$1" >/dev/null && cd "$2/firmware" && idf.py build && "$2/tools/flash.sh"`
	cmd := exec.Command("bash", "-c", script, "bash", idf, RepoDir)
	out, _ := cmd.StdoutPipe()
	cmd.Stderr = cmd.Stdout
	if err := cmd.Start(); err != nil {
		job.mu.Lock()
		job.running = false
		job.mu.Unlock()
		jsonErr(w, http.StatusInternalServerError, err.Error(), "")
		return
	}
	go func() {
		sc := bufio.NewScanner(out)
		sc.Buffer(make([]byte, 4096), 1<<20)
		for sc.Scan() {
			line := strings.TrimRight(sc.Text(), "\r")
			if i := strings.LastIndex(line, "\r"); i >= 0 { // esptool progress bars
				line = line[i+1:]
			}
			if line == "" {
				continue
			}
			b, _ := json.Marshal(map[string]any{"line": line})
			s.broadcastEvent("flash", b)
		}
		err := cmd.Wait()
		b, _ := json.Marshal(map[string]any{"done": true, "ok": err == nil})
		s.broadcastEvent("flash", b)
		job.mu.Lock()
		job.running = false
		job.mu.Unlock()
	}()
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"ok":true}`))
}
