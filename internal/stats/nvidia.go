package stats

import (
	"bufio"
	"io"
	"log"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// NVIDIA's proprietary driver exposes no load, VRAM or temperature in sysfs; NVML is
// the only source. Two ways to reach it without cgo were weighed:
//
//   - NVML in-process through purego (dlopen of libnvidia-ml.so.1). Cheapest per
//     sample, but it needs purego's fake-cgo runtime shim in a CGO_ENABLED=0 build,
//     which changes how the whole daemon starts for every user (AMD and Intel ones
//     too), loads a ~2 MB driver library plus its threads into the daemon for good
//     (dlclose doesn't really unload it), and lets a driver crash or hang take the
//     daemon down.
//   - A long-lived `nvidia-smi --query-gpu=… -lms 1000` child. One process start per
//     visit of a stats page (not one per second), one CSV line per second on a pipe,
//     isolated from the daemon, and nothing at all on machines without NVIDIA or when
//     no GPU gauge is on screen.
//
// The child wins: zero idle cost and no new dependency. It is started by the first
// Sample that wants a GPU key and killed by Stop (the daemon stops stats when the
// page goes away or the screen sleeps). It also dies with the daemon (Pdeathsig).

const nvidiaQuery = "utilization.gpu,memory.used,memory.total,temperature.gpu"

type nvidiaSMI struct {
	bin  string
	args []string

	mu      sync.Mutex
	cmd     *exec.Cmd
	last    nvidiaValues
	lastAt  time.Time
	retryAt time.Time // after an unexpected exit, don't respawn before this
}

type nvidiaValues struct {
	util, memUsed, memTotal, temp int64 // -1 when not supported
}

func newNvidiaSMI(g GPU) *nvidiaSMI {
	id := g.PCI
	if len(id) == 12 { // 0000:01:00.0 -> nvidia-smi's 00000000:01:00.0
		id = "0000" + id
	}
	return &nvidiaSMI{bin: "nvidia-smi", args: []string{"--query-gpu=" + nvidiaQuery, "--format=csv,noheader,nounits", "-lms", "1000", "--id=" + id}}
}

func (n *nvidiaSMI) sample(want map[string]bool, s Sample) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.cmd == nil && time.Now().After(n.retryAt) {
		n.start()
	}
	if n.lastAt.IsZero() || time.Since(n.lastAt) > 5*time.Second {
		return // not started yet, or stuck
	}
	v := n.last
	if want["gpu"] && v.util >= 0 {
		s["gpu"] = int(v.util)
	}
	if want["vram"] && v.memUsed >= 0 && v.memTotal > 0 {
		s["vram"] = int(100 * v.memUsed / v.memTotal)
	}
	if want["gpuTemp"] && v.temp >= 0 {
		s["gpuTemp"] = int(v.temp)
	}
}

// start runs the child; n.mu is held.
func (n *nvidiaSMI) start() {
	cmd := exec.Command(n.bin, n.args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGKILL}
	out, err := cmd.StdoutPipe()
	if err == nil {
		err = cmd.Start()
	}
	if err != nil {
		log.Printf("stats: %s: %v", n.bin, err)
		n.retryAt = time.Now().Add(time.Minute)
		return
	}
	n.cmd = cmd
	go n.read(cmd, out)
}

func (n *nvidiaSMI) read(cmd *exec.Cmd, out io.Reader) {
	sc := bufio.NewScanner(out)
	for sc.Scan() {
		if v, ok := parseNvidiaLine(sc.Text()); ok {
			n.mu.Lock()
			if n.cmd == cmd {
				n.last, n.lastAt = v, time.Now()
			}
			n.mu.Unlock()
		}
	}
	err := cmd.Wait()
	n.mu.Lock()
	if n.cmd == cmd { // not stopped on purpose
		log.Printf("stats: %s exited: %v", n.bin, err)
		n.cmd, n.lastAt = nil, time.Time{}
		n.retryAt = time.Now().Add(30 * time.Second)
	}
	n.mu.Unlock()
}

func (n *nvidiaSMI) stop() {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.cmd != nil {
		_ = n.cmd.Process.Kill() // read() reaps it
		n.cmd, n.lastAt = nil, time.Time{}
	}
	n.retryAt = time.Time{}
}

// parseNvidiaLine reads "37, 2190, 8192, 54" (fields as in nvidiaQuery). Fields can
// be "[N/A]" or "[Not Supported]".
func parseNvidiaLine(line string) (nvidiaValues, bool) {
	f := strings.Split(line, ",")
	if len(f) != 4 {
		return nvidiaValues{}, false
	}
	num := func(s string) int64 {
		v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
		if err != nil {
			return -1
		}
		return int64(v + 0.5)
	}
	v := nvidiaValues{num(f[0]), num(f[1]), num(f[2]), num(f[3])}
	if v.util < 0 && v.memUsed < 0 && v.temp < 0 {
		return v, false
	}
	return v, true
}
