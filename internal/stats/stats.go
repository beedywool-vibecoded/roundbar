//go:build linux

// Package stats reads CPU, RAM and GPU figures straight from procfs/sysfs (and, for
// NVIDIA, from an nvidia-smi child that only runs while GPU figures are wanted).
// Files are opened once and re-read with pread, so a sample costs a few syscalls.
package stats

import (
	"bytes"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

type Reader struct {
	mu                  sync.Mutex
	root                string // "/" except in tests
	stat, meminfo       *os.File
	cpuTemp             *os.File
	gpu                 gpuSource
	GPU                 GPU // the GPU gpu/vram/gpuTemp come from (zero if none)
	prevIdle, prevTotal uint64
	buf                 []byte
}

func open(p string) *os.File {
	if p == "" {
		return nil
	}
	f, err := os.Open(p)
	if err != nil {
		return nil
	}
	return f
}

// hwmonInput finds tempN_input of the first hwmon whose name is in names,
// preferring the sensor labelled one of labels.
func hwmonInput(base string, names, labels []string) string {
	dirs, _ := filepath.Glob(filepath.Join(base, "hwmon[0-9]*"))
	sub, _ := filepath.Glob(filepath.Join(base, "hwmon", "hwmon[0-9]*")) // drm devices nest one level
	dirs = append(dirs, sub...)
	for _, n := range names {
		for _, d := range dirs {
			name, _ := os.ReadFile(filepath.Join(d, "name"))
			if strings.TrimSpace(string(name)) != n {
				continue
			}
			for _, l := range labels {
				ls, _ := filepath.Glob(filepath.Join(d, "temp*_label"))
				for _, lf := range ls {
					v, _ := os.ReadFile(lf)
					if strings.TrimSpace(string(v)) == l {
						return strings.TrimSuffix(lf, "_label") + "_input"
					}
				}
			}
			if _, err := os.Stat(filepath.Join(d, "temp1_input")); err == nil {
				return filepath.Join(d, "temp1_input")
			}
		}
	}
	return ""
}

func readInt(p string) int64 {
	b, err := os.ReadFile(p)
	if err != nil {
		return -1
	}
	n, err := strconv.ParseInt(strings.TrimSpace(string(b)), 10, 64)
	if err != nil {
		return -1
	}
	return n
}

func New() *Reader { return newReader("/", "") }

// newReader reads below root (a fake sysfs/procfs tree in tests). gpuSel picks a
// GPU by card name, PCI address or driver; "" picks the best one (see detectGPUs).
func newReader(root, gpuSel string) *Reader {
	r := &Reader{root: root, buf: make([]byte, 4096)}
	r.stat = open(filepath.Join(root, "proc/stat"))
	r.meminfo = open(filepath.Join(root, "proc/meminfo"))
	r.cpuTemp = open(hwmonInput(filepath.Join(root, "sys/class/hwmon"), []string{"k10temp", "zenpower", "coretemp"}, []string{"Tctl", "Tdie", "Package id 0"}))
	if g, ok := pickGPU(detectGPUs(root), gpuSel); ok {
		r.GPU = g
		r.gpu = newGPUSource(root, g)
		if r.gpu != nil {
			log.Printf("stats: GPU %s (%s, %s)", g.PCI, g.Driver, g.Card)
		}
	}
	return r
}

func (r *Reader) read(f *os.File) []byte {
	if f == nil {
		return nil
	}
	n, err := f.ReadAt(r.buf, 0)
	if n == 0 && err != nil {
		return nil
	}
	return r.buf[:n]
}

// Sample returns the requested keys only. CPU load is relative to the previous call.
func (r *Reader) Sample(want map[string]bool) Sample {
	r.mu.Lock()
	defer r.mu.Unlock()
	s := Sample{}
	if want["cpu"] {
		if b := r.read(r.stat); b != nil {
			line, _, _ := bytes.Cut(b, []byte{'\n'})
			f := strings.Fields(string(line))
			var total, idle uint64
			for i, v := range f[1:] {
				n, _ := strconv.ParseUint(v, 10, 64)
				total += n
				if i == 3 || i == 4 { // idle + iowait
					idle += n
				}
			}
			if dt := total - r.prevTotal; r.prevTotal != 0 && dt > 0 {
				s["cpu"] = int(100 * (dt - (idle - r.prevIdle)) / dt)
			}
			r.prevIdle, r.prevTotal = idle, total
		}
	}
	if want["ram"] {
		if b := r.read(r.meminfo); b != nil {
			var total, avail int64
			for _, line := range strings.Split(string(b), "\n") {
				f := strings.Fields(line)
				if len(f) < 2 {
					continue
				}
				switch f[0] {
				case "MemTotal:":
					total, _ = strconv.ParseInt(f[1], 10, 64)
				case "MemAvailable:":
					avail, _ = strconv.ParseInt(f[1], 10, 64)
				}
			}
			if total > 0 {
				s["ram"] = int(100 * (total - avail) / total)
			}
		}
	}
	if want["cpuTemp"] {
		if v, ok := readFileInt(r.cpuTemp, r.buf); ok {
			s["cpuTemp"] = int(v / 1000)
		}
	}
	if r.gpu != nil && (want["gpu"] || want["vram"] || want["gpuTemp"]) {
		r.gpu.sample(want, s)
	}
	return s
}

// Stop ends background helpers (the nvidia-smi child). Called when no page needs
// stats any more; the next Sample starts them again.
func (r *Reader) Stop() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.gpu != nil {
		r.gpu.stop()
	}
}

// readFileInt preads an integer file kept open.
func readFileInt(f *os.File, buf []byte) (int64, bool) {
	if f == nil {
		return 0, false
	}
	n, err := f.ReadAt(buf, 0)
	if n == 0 && err != nil {
		return 0, false
	}
	v, err := strconv.ParseInt(string(bytes.TrimSpace(buf[:n])), 10, 64)
	return v, err == nil
}
