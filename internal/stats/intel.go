//go:build linux

package stats

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Intel (i915 and xe) has no busy-percent file. The engine busyness PMU needs
// CAP_PERFMON or perf_event_paranoid <= 0, so instead the load comes from DRM
// fdinfo, which every process exposes for its own GPU clients (kernel >= 5.19 for
// i915, any xe): i915 reports busy nanoseconds per engine, xe busy cycles against
// a GPU timestamp. Only the user's own processes are readable without root, which
// on a desktop session covers the compositor and the apps (an X server running as
// root is missed). When no client is readable the load falls back to the current
// frequency over the maximum, a rough proxy. VRAM is not
// reported (integrated GPUs share system RAM); the temperature comes from hwmon,
// which discrete cards expose.

const intelRescan = 10 // samples between scans of /proc for new GPU clients

type intel struct {
	root, pci, driver string
	fdinfo            []string // /proc/<pid>/fdinfo/<fd> of clients on this GPU
	samples           int
	prev              map[string]engineCounters // by drm-client-id
	prevAt            time.Time
	sawClients        bool
	actFreq, maxFreq  string
	temp              *os.File
	buf               []byte
}

// engine -> busy counter, plus "total-" engine entries for xe's cycle reference.
type engineCounters map[string]uint64

func newIntel(root string, g GPU) *intel {
	in := &intel{root: root, pci: g.PCI, driver: g.Driver, prev: map[string]engineCounters{}, buf: make([]byte, 64)}
	card := filepath.Join(root, "sys/class/drm", g.Card)
	if g.Driver == "i915" {
		in.actFreq, in.maxFreq = filepath.Join(card, "gt_act_freq_mhz"), filepath.Join(card, "gt_RP0_freq_mhz")
	} else {
		gt := filepath.Join(g.Dev, "tile0/gt0/freq0")
		in.actFreq, in.maxFreq = filepath.Join(gt, "act_freq"), filepath.Join(gt, "rp0_freq")
	}
	in.temp = open(hwmonInput(g.Dev, []string{g.Driver}, []string{"pkg", "vram"}))
	return in
}

func (in *intel) sample(want map[string]bool, s Sample) {
	if want["gpu"] {
		if v, ok := in.load(); ok {
			s["gpu"] = v
		}
	}
	if want["gpuTemp"] {
		if v, ok := readFileInt(in.temp, in.buf); ok {
			s["gpuTemp"] = int(v / 1000)
		}
	}
}

func (in *intel) stop() {
	in.prev, in.fdinfo, in.samples = map[string]engineCounters{}, nil, 0
}

func (in *intel) load() (int, bool) {
	if in.samples%intelRescan == 0 {
		in.fdinfo = in.scanClients()
	}
	in.samples++
	now := time.Now()
	cur := map[string]engineCounters{}
	for _, p := range in.fdinfo {
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		id, pdev, c := parseFdinfo(string(b))
		if pdev == in.pci && id != "" && len(c) > 0 {
			cur[id] = c
		}
	}
	if len(cur) > 0 {
		in.sawClients = true
	}
	prev, dt := in.prev, now.Sub(in.prevAt)
	in.prev, in.prevAt = cur, now
	if !in.sawClients {
		act, mx := readInt(in.actFreq), readInt(in.maxFreq)
		if act < 0 || mx <= 0 {
			return 0, false
		}
		return int(min(100, 100*act/mx)), true
	}
	if len(prev) == 0 || dt <= 0 {
		return 0, false // first sample after (re)start: no delta yet
	}
	return engineLoad(prev, cur, dt), true
}

// engineLoad is the busiest engine's utilisation in percent, summed over the
// clients present in both samples.
func engineLoad(prev, cur map[string]engineCounters, dt time.Duration) int {
	busy := map[string]float64{} // engine -> fraction
	for id, c := range cur {
		p, ok := prev[id]
		if !ok {
			continue
		}
		for k, v := range c {
			if strings.HasPrefix(k, "total-") || strings.HasPrefix(k, "capacity-") || v < p[k] {
				continue
			}
			d := float64(v - p[k])
			if tot, ok := c["total-"+k]; ok { // xe: cycles against the GPU timestamp
				dtot := float64(tot - p["total-"+k])
				if tot <= p["total-"+k] {
					continue
				}
				busy[k] += d / dtot
			} else { // i915: nanoseconds
				busy[k] += d / float64(dt.Nanoseconds())
			}
		}
	}
	best := 0.0
	for k, f := range busy {
		if capa := cur0(cur, "capacity-"+k); capa > 1 {
			f /= float64(capa)
		}
		best = max(best, f)
	}
	return int(min(100, best*100+0.5))
}

func cur0(m map[string]engineCounters, k string) uint64 {
	for _, c := range m {
		if v, ok := c[k]; ok {
			return v
		}
	}
	return 0
}

// parseFdinfo returns the client id, PCI device and engine counters of a DRM fdinfo
// file: i915 "drm-engine-render: 123 ns", xe "drm-cycles-rcs: 1" and
// "drm-total-cycles-rcs: 2", both "drm-engine-capacity-video: 2".
func parseFdinfo(s string) (id, pdev string, c engineCounters) {
	c = engineCounters{}
	for _, line := range strings.Split(s, "\n") {
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		v = strings.TrimSpace(v)
		num := func() (uint64, bool) {
			f, _, _ := strings.Cut(v, " ")
			n, err := strconv.ParseUint(f, 10, 64)
			return n, err == nil
		}
		switch {
		case k == "drm-client-id":
			id = v
		case k == "drm-pdev":
			pdev = strings.ToLower(v)
		case strings.HasPrefix(k, "drm-engine-capacity-"):
			if n, ok := num(); ok {
				c["capacity-"+strings.TrimPrefix(k, "drm-engine-capacity-")] = n
			}
		case strings.HasPrefix(k, "drm-engine-"):
			if n, ok := num(); ok {
				c[strings.TrimPrefix(k, "drm-engine-")] = n
			}
		case strings.HasPrefix(k, "drm-total-cycles-"):
			if n, ok := num(); ok {
				c["total-"+strings.TrimPrefix(k, "drm-total-cycles-")] = n
			}
		case strings.HasPrefix(k, "drm-cycles-"):
			if n, ok := num(); ok {
				c[strings.TrimPrefix(k, "drm-cycles-")] = n
			}
		}
	}
	return id, pdev, c
}

// scanClients walks /proc for file descriptors open on a DRM device node and keeps
// those whose fdinfo names this GPU. Other users' processes are not readable and
// are skipped.
func (in *intel) scanClients() []string {
	var out []string
	procs, _ := os.ReadDir(filepath.Join(in.root, "proc"))
	for _, p := range procs {
		if p.Name()[0] < '0' || p.Name()[0] > '9' {
			continue
		}
		fdDir := filepath.Join(in.root, "proc", p.Name(), "fd")
		fds, err := os.ReadDir(fdDir)
		if err != nil {
			continue
		}
		for _, fd := range fds {
			t, err := os.Readlink(filepath.Join(fdDir, fd.Name()))
			if err != nil || !strings.HasPrefix(t, "/dev/dri/") {
				continue
			}
			info := filepath.Join(in.root, "proc", p.Name(), "fdinfo", fd.Name())
			b, err := os.ReadFile(info)
			if err != nil {
				continue
			}
			if _, pdev, c := parseFdinfo(string(b)); pdev == in.pci && len(c) > 0 {
				out = append(out, info)
			}
		}
	}
	return out
}
