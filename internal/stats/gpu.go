//go:build linux

package stats

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// GPU is one graphics card found in sysfs.
type GPU struct {
	Card     string // "card1"; empty for an NVIDIA GPU seen only in /proc/driver/nvidia
	Driver   string // amdgpu, i915, xe, nvidia, nouveau
	PCI      string // "0000:03:00.0"
	Dev      string // sysfs device directory
	VRAM     int64  // bytes, 0 when unknown
	Discrete bool
	Display  bool // drives at least one connected output
}

// detectGPUs lists the GPUs, best first: discrete before integrated, then the most
// VRAM, then the one driving a display.
func detectGPUs(root string) []GPU {
	var out []GPU
	seen := map[string]bool{}
	drm := filepath.Join(root, "sys/class/drm")
	cards, _ := filepath.Glob(filepath.Join(drm, "card[0-9]*"))
	for _, c := range cards {
		name := filepath.Base(c)
		if strings.Contains(name, "-") { // connectors: card1-DP-1
			continue
		}
		dev := filepath.Join(c, "device")
		drv, err := os.Readlink(filepath.Join(dev, "driver"))
		if err != nil {
			continue
		}
		g := GPU{Card: name, Driver: filepath.Base(drv), Dev: dev}
		if real, err := filepath.EvalSymlinks(dev); err == nil {
			g.PCI = filepath.Base(real)
		}
		if g.PCI == "" || seen[g.PCI] {
			continue
		}
		conns, _ := filepath.Glob(filepath.Join(drm, name+"-*", "status"))
		for _, s := range conns {
			if b, _ := os.ReadFile(s); strings.TrimSpace(string(b)) == "connected" {
				g.Display = true
			}
		}
		switch g.Driver {
		case "amdgpu":
			g.VRAM = max(readInt(filepath.Join(dev, "mem_info_vram_total")), 0)
			// APUs have a small carve-out and no VRAM vendor; dGPUs report one.
			_, err := os.Stat(filepath.Join(dev, "mem_info_vram_vendor"))
			g.Discrete = err == nil || g.VRAM >= 2<<30
		case "i915", "xe":
			g.Discrete = !strings.HasSuffix(g.PCI, ":00:02.0") // Intel iGPUs always sit at 00:02.0
		case "nvidia", "nouveau":
			g.Discrete = true
		default:
			continue
		}
		seen[g.PCI] = true
		out = append(out, g)
	}
	// The proprietary driver without nvidia-drm has no DRM card.
	nv, _ := filepath.Glob(filepath.Join(root, "proc/driver/nvidia/gpus/*"))
	for _, d := range nv {
		pci := strings.ToLower(filepath.Base(d))
		if !seen[pci] {
			seen[pci] = true
			out = append(out, GPU{Driver: "nvidia", PCI: pci, Discrete: true})
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Discrete != b.Discrete {
			return a.Discrete
		}
		if a.VRAM != b.VRAM {
			return a.VRAM > b.VRAM
		}
		if a.Display != b.Display {
			return a.Display
		}
		return a.PCI < b.PCI
	})
	return out
}

// pickGPU returns the GPU matching sel (card name, PCI address or driver), or the
// first one when sel is empty. Kept internal until the config grows a GPU choice.
func pickGPU(gpus []GPU, sel string) (GPU, bool) {
	for _, g := range gpus {
		if sel == "" || g.Card == sel || g.PCI == sel || g.Driver == sel {
			return g, true
		}
	}
	return GPU{}, false
}

func newGPUSource(root string, g GPU) gpuSource {
	switch g.Driver {
	case "amdgpu":
		return newAMD(g)
	case "i915", "xe":
		return newIntel(root, g)
	case "nvidia":
		return newNvidiaSMI(g)
	case "nouveau": // no load or VRAM without root, but the temperature is in hwmon
		return &tempOnly{temp: open(hwmonInput(g.Dev, []string{"nouveau"}, nil)), buf: make([]byte, 64)}
	}
	return nil
}

func newNvidiaSMI(g GPU) *nvidiaSMI {
	id := g.PCI
	if len(id) == 12 { // 0000:01:00.0 -> nvidia-smi's 00000000:01:00.0
		id = "0000" + id
	}
	return newNvidiaQuery(id)
}

// ---- AMD ----------------------------------------------------------------

type amd struct {
	busy, vramUsed, temp *os.File
	vramTotal            int64
	buf                  []byte
}

func newAMD(g GPU) *amd {
	return &amd{
		busy:      open(filepath.Join(g.Dev, "gpu_busy_percent")),
		vramUsed:  open(filepath.Join(g.Dev, "mem_info_vram_used")),
		vramTotal: g.VRAM,
		temp:      open(hwmonInput(g.Dev, []string{"amdgpu"}, []string{"junction", "edge"})),
		buf:       make([]byte, 64),
	}
}

func (a *amd) sample(want map[string]bool, s Sample) {
	if want["gpu"] {
		if v, ok := readFileInt(a.busy, a.buf); ok {
			s["gpu"] = int(v)
		}
	}
	if want["vram"] && a.vramTotal > 0 {
		if v, ok := readFileInt(a.vramUsed, a.buf); ok {
			s["vram"] = int(100 * v / a.vramTotal)
		}
	}
	if want["gpuTemp"] {
		if v, ok := readFileInt(a.temp, a.buf); ok {
			s["gpuTemp"] = int(v / 1000)
		}
	}
}

func (a *amd) stop() {}

// ---- temperature only ---------------------------------------------------

type tempOnly struct {
	temp *os.File
	buf  []byte
}

func (t *tempOnly) sample(want map[string]bool, s Sample) {
	if want["gpuTemp"] {
		if v, ok := readFileInt(t.temp, t.buf); ok {
			s["gpuTemp"] = int(v / 1000)
		}
	}
}

func (t *tempOnly) stop() {}
