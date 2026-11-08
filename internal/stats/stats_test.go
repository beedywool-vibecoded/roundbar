//go:build linux

package stats

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

var all = map[string]bool{"cpu": true, "ram": true, "gpu": true, "vram": true, "cpuTemp": true, "gpuTemp": true}

// On the real machine: whatever it has, RAM is always readable.
func TestSampleHere(t *testing.T) {
	r := New()
	defer r.Stop()
	r.Sample(all)
	time.Sleep(300 * time.Millisecond)
	s := r.Sample(all)
	t.Logf("GPU %+v: %v", r.GPU, s)
	if _, ok := s["ram"]; !ok {
		t.Fatal("no ram")
	}
}

// fakeRoot builds a sysfs/procfs tree: files maps paths to contents, links maps
// paths to symlink targets.
func fakeRoot(t *testing.T, files, links map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for p, c := range files {
		full := filepath.Join(root, p)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(c), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for p, target := range links {
		full := filepath.Join(root, p)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, full); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// card adds a DRM card whose device is the PCI dir pci, bound to driver.
func card(files, links map[string]string, name, pci, driver string, dev map[string]string, connected bool) {
	devDir := "sys/devices/pci0000:00/" + pci
	links["sys/class/drm/"+name+"/device"] = "../../../devices/pci0000:00/" + pci
	links[devDir+"/driver"] = "../../../bus/pci/drivers/" + driver
	files["sys/class/drm/"+name+"/dev"] = "226:0"
	for k, v := range dev {
		files[devDir+"/"+k] = v
	}
	status := "disconnected"
	if connected {
		status = "connected"
	}
	files["sys/class/drm/"+name+"-DP-1/status"] = status
}

func TestAMDPicksDiscrete(t *testing.T) {
	files, links := map[string]string{
		"proc/stat":    "cpu  100 0 100 800 0 0 0 0 0 0\n",
		"proc/meminfo": "MemTotal: 1000 kB\nMemFree: 100 kB\nMemAvailable: 250 kB\n",
	}, map[string]string{}
	// APU drives the display, but the discrete card wins.
	card(files, links, "card0", "0000:12:00.0", "amdgpu", map[string]string{
		"gpu_busy_percent": "1\n", "mem_info_vram_total": "536870912\n", "mem_info_vram_used": "100\n",
	}, true)
	card(files, links, "card1", "0000:03:00.0", "amdgpu", map[string]string{
		"gpu_busy_percent": "42\n", "mem_info_vram_total": "8000\n", "mem_info_vram_used": "2000\n", "mem_info_vram_vendor": "samsung\n",
		"hwmon/hwmon3/name": "amdgpu\n", "hwmon/hwmon3/temp1_input": "50000\n", "hwmon/hwmon3/temp1_label": "edge\n",
		"hwmon/hwmon3/temp2_input": "61000\n", "hwmon/hwmon3/temp2_label": "junction\n",
	}, false)
	r := newReader(fakeRoot(t, files, links), "")
	if r.GPU.PCI != "0000:03:00.0" || !r.GPU.Discrete {
		t.Fatalf("picked %+v", r.GPU)
	}
	s := r.Sample(all)
	if s["gpu"] != 42 || s["vram"] != 25 || s["gpuTemp"] != 61 || s["ram"] != 75 {
		t.Fatalf("sample %v", s)
	}
	// The selector can still pick the APU.
	r = newReader(fakeRoot(t, files, links), "card0")
	if s := r.Sample(map[string]bool{"gpu": true}); r.GPU.Card != "card0" || s["gpu"] != 1 {
		t.Fatalf("selector: %+v %v", r.GPU, s)
	}
}

func TestDetectOrder(t *testing.T) {
	files, links := map[string]string{}, map[string]string{}
	card(files, links, "card0", "0000:00:02.0", "i915", nil, true)
	card(files, links, "card1", "0000:01:00.0", "nvidia", nil, false)
	files["proc/driver/nvidia/gpus/0000:01:00.0/information"] = "Model: x\n"
	files["proc/driver/nvidia/gpus/0000:02:00.0/information"] = "Model: y\n" // no nvidia-drm card
	gpus := detectGPUs(fakeRoot(t, files, links))
	var got []string
	for _, g := range gpus {
		got = append(got, g.Driver+"@"+g.PCI)
	}
	want := "nvidia@0000:01:00.0 nvidia@0000:02:00.0 i915@0000:00:02.0"
	if strings.Join(got, " ") != want {
		t.Fatalf("got %v, want %s", got, want)
	}
	if g, _ := pickGPU(gpus, "i915"); g.Card != "card0" {
		t.Fatalf("pick by driver: %+v", g)
	}
}

func TestIntelXeFdinfo(t *testing.T) {
	files, links := map[string]string{}, map[string]string{}
	card(files, links, "card0", "0000:03:00.0", "xe", map[string]string{
		"hwmon/hwmon5/name": "xe\n", "hwmon/hwmon5/temp2_input": "48000\n", "hwmon/hwmon5/temp2_label": "pkg\n",
	}, true)
	fdinfo := func(cycles, total uint64) string {
		return "pos:\t0\ndrm-driver:\txe\ndrm-client-id:\t21\ndrm-pdev:\t0000:03:00.0\n" +
			"drm-cycles-rcs:\t" + itoa(cycles) + "\ndrm-total-cycles-rcs:\t" + itoa(total) + "\n" +
			"drm-cycles-vcs:\t0\ndrm-total-cycles-vcs:\t" + itoa(total) + "\ndrm-engine-capacity-vcs:\t2\n"
	}
	files["proc/4242/fdinfo/7"] = fdinfo(1000, 10000)
	links["proc/4242/fd/7"] = "/dev/dri/renderD128"
	files["proc/4242/fdinfo/3"] = "pos:\t0\n" // not a DRM fd
	links["proc/4242/fd/3"] = "/dev/null"
	files["proc/77/fdinfo/9"] = strings.ReplaceAll(fdinfo(5, 10), "0000:03:00.0", "0000:00:02.0") // another GPU
	links["proc/77/fd/9"] = "/dev/dri/renderD129"
	root := fakeRoot(t, files, links)
	r := newReader(root, "")
	s := r.Sample(all)
	if _, ok := s["gpu"]; ok || s["gpuTemp"] != 48 {
		t.Fatalf("first sample has no delta yet: %v", s)
	}
	if _, ok := s["vram"]; ok {
		t.Fatalf("no VRAM on Intel: %v", s)
	}
	// 3000 busy cycles out of 5000.
	if err := os.WriteFile(filepath.Join(root, "proc/4242/fdinfo/7"), []byte(fdinfo(4000, 15000)), 0o644); err != nil {
		t.Fatal(err)
	}
	if s := r.Sample(all); s["gpu"] != 60 {
		t.Fatalf("want 60 %%, got %v", s)
	}
}

func TestIntelFrequencyFallback(t *testing.T) {
	files, links := map[string]string{}, map[string]string{}
	card(files, links, "card0", "0000:00:02.0", "i915", nil, true)
	files["sys/class/drm/card0/gt_act_freq_mhz"] = "650\n"
	files["sys/class/drm/card0/gt_RP0_freq_mhz"] = "1300\n"
	r := newReader(fakeRoot(t, files, links), "")
	if s := r.Sample(all); s["gpu"] != 50 {
		t.Fatalf("want 50 %% from the frequency, got %v", s)
	}
	if _, ok := r.Sample(all)["gpuTemp"]; ok {
		t.Fatal("an iGPU has no temperature sensor here")
	}
}

func TestI915EngineLoad(t *testing.T) {
	_, _, c0 := parseFdinfo("drm-driver:\ti915\ndrm-client-id:\t7\ndrm-pdev:\t0000:00:02.0\n" +
		"drm-engine-render:\t1000000000 ns\ndrm-engine-copy:\t0 ns\ndrm-engine-video:\t0 ns\ndrm-engine-capacity-video:\t2\n")
	_, _, c1 := parseFdinfo("drm-driver:\ti915\ndrm-client-id:\t7\ndrm-pdev:\t0000:00:02.0\n" +
		"drm-engine-render:\t1250000000 ns\ndrm-engine-copy:\t0 ns\ndrm-engine-video:\t500000000 ns\ndrm-engine-capacity-video:\t2\n")
	_, _, other := parseFdinfo("drm-client-id:\t8\ndrm-engine-render:\t100000000 ns\n")
	prev := map[string]engineCounters{"7": c0, "8": other}
	cur := map[string]engineCounters{"7": c1, "9": other} // 8 closed, 9 is new: both ignored
	// render: 250 ms busy in 500 ms = 50 %; video: 500 ms over 2 engines = 50 %.
	if got := engineLoad(prev, cur, 500*time.Millisecond); got != 50 {
		t.Fatalf("got %d", got)
	}
	if got := engineLoad(prev, cur, 200*time.Millisecond); got != 100 {
		t.Fatalf("capped: got %d", got)
	}
}

func TestNouveauTempOnly(t *testing.T) {
	files, links := map[string]string{}, map[string]string{}
	card(files, links, "card0", "0000:01:00.0", "nouveau", map[string]string{
		"hwmon/hwmon2/name": "nouveau\n", "hwmon/hwmon2/temp1_input": "39000\n",
	}, true)
	s := newReader(fakeRoot(t, files, links), "").Sample(all)
	if s["gpuTemp"] != 39 {
		t.Fatalf("%v", s)
	}
	if _, ok := s["gpu"]; ok {
		t.Fatalf("nouveau has no load: %v", s)
	}
}

func TestParseNvidiaRecorded(t *testing.T) {
	b, err := os.ReadFile("testdata/nvidia-smi.txt")
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	want := []struct {
		v  nvidiaValues
		ok bool
	}{
		{nvidiaValues{3, 1043, 8188, 41}, true},
		{nvidiaValues{37, 2190, 8188, 54}, true},
		{nvidiaValues{100, 7912, 8188, 79}, true},
		{nvidiaValues{-1, 512, 4096, -1}, true},
		{nvidiaValues{-1, -1, -1, -1}, false},
	}
	for i, l := range lines {
		v, ok := parseNvidiaLine(l)
		if v != want[i].v || ok != want[i].ok {
			t.Errorf("%q: got %+v %v", l, v, ok)
		}
	}
}

// The child starts on the first Sample that wants a GPU key and is killed by Stop.
func TestNvidiaChild(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "nvidia-smi")
	script := "#!/bin/sh\necho \"$@\" > " + filepath.Join(dir, "args") + "\nwhile :; do echo '37, 2048, 8192, 54'; sleep 0.1; done\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	n := newNvidiaSMI(GPU{PCI: "0000:01:00.0"})
	n.bin = bin
	n.sample(map[string]bool{"ram": true}, Sample{})
	var s Sample
	for i := 0; i < 50; i++ {
		s = Sample{}
		n.sample(all, s)
		if len(s) > 0 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if s["gpu"] != 37 || s["vram"] != 25 || s["gpuTemp"] != 54 {
		t.Fatalf("sample %v", s)
	}
	args, _ := os.ReadFile(filepath.Join(dir, "args"))
	if !strings.Contains(string(args), "--id=00000000:01:00.0") || !strings.Contains(string(args), "-lms 1000") {
		t.Fatalf("args %q", args)
	}
	n.mu.Lock()
	p := n.cmd.Process
	n.mu.Unlock()
	n.stop()
	time.Sleep(200 * time.Millisecond) // read() reaps it
	if _, err := os.Stat(filepath.Join("/proc", strconv.Itoa(p.Pid))); err == nil {
		t.Fatal("nvidia-smi still running after Stop")
	}
	n.mu.Lock()
	running := n.cmd != nil
	n.mu.Unlock()
	if running {
		t.Fatal("stop left the command set")
	}
}

func itoa(n uint64) string { return strconv.FormatUint(n, 10) }
