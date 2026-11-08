// Package stats reads CPU, RAM and AMD GPU figures straight from procfs/sysfs.
// Files are opened once and re-read with pread, so a sample costs a few syscalls.
package stats

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Sample map[string]int // keys: cpu ram gpu vram cpuTemp gpuTemp

type Reader struct {
	stat, meminfo       *os.File
	cpuTemp, gpuTemp    *os.File
	gpuBusy, vramUsed   *os.File
	vramTotal           int64
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

func amdGPU() string {
	cards, _ := filepath.Glob("/sys/class/drm/card[0-9]")
	best, bestVram := "", int64(-1)
	for _, c := range cards {
		dev := filepath.Join(c, "device")
		if _, err := os.Stat(filepath.Join(dev, "gpu_busy_percent")); err != nil {
			continue
		}
		v := readInt(filepath.Join(dev, "mem_info_vram_total"))
		if v > bestVram { // a discrete GPU has more VRAM than an iGPU
			best, bestVram = dev, v
		}
	}
	return best
}

func readInt(p string) int64 {
	b, err := os.ReadFile(p)
	if err != nil {
		return -1
	}
	n, _ := strconv.ParseInt(strings.TrimSpace(string(b)), 10, 64)
	return n
}

func New() *Reader {
	r := &Reader{buf: make([]byte, 4096)}
	r.stat = open("/proc/stat")
	r.meminfo = open("/proc/meminfo")
	r.cpuTemp = open(hwmonInput("/sys/class/hwmon", []string{"k10temp", "zenpower", "coretemp"}, []string{"Tctl", "Tdie", "Package id 0"}))
	if g := amdGPU(); g != "" {
		r.gpuBusy = open(filepath.Join(g, "gpu_busy_percent"))
		r.vramUsed = open(filepath.Join(g, "mem_info_vram_used"))
		r.vramTotal = readInt(filepath.Join(g, "mem_info_vram_total"))
		r.gpuTemp = open(hwmonInput(g, []string{"amdgpu"}, []string{"junction", "edge"}))
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

func (r *Reader) int(f *os.File) (int64, bool) {
	b := r.read(f)
	if b == nil {
		return 0, false
	}
	n, err := strconv.ParseInt(string(bytes.TrimSpace(b)), 10, 64)
	return n, err == nil
}

// Sample returns the requested keys only. CPU load is relative to the previous call.
func (r *Reader) Sample(want map[string]bool) Sample {
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
		if v, ok := r.int(r.cpuTemp); ok {
			s["cpuTemp"] = int(v / 1000)
		}
	}
	if want["gpu"] {
		if v, ok := r.int(r.gpuBusy); ok {
			s["gpu"] = int(v)
		}
	}
	if want["vram"] && r.vramTotal > 0 {
		if v, ok := r.int(r.vramUsed); ok {
			s["vram"] = int(100 * v / r.vramTotal)
		}
	}
	if want["gpuTemp"] {
		if v, ok := r.int(r.gpuTemp); ok {
			s["gpuTemp"] = int(v / 1000)
		}
	}
	return s
}
