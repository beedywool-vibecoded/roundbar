// Package stats reads CPU, RAM and GPU figures. On Windows: GetSystemTimes,
// GlobalMemoryStatusEx, the GPU performance counters (PDH) of the adapter
// with the most dedicated memory, or for NVIDIA cards an nvidia-smi child that
// only runs while GPU figures are wanted (it also gives the temperature).
// CPU temperature needs a kernel driver on Windows and is left out.
package stats

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"unsafe"

	"github.com/beedywool-vibecoded/roundbar/internal/win"
	"golang.org/x/sys/windows"
)

var (
	procGetSystemTimes       = win.Kernel32("GetSystemTimes")
	procGlobalMemoryStatusEx = win.Kernel32("GlobalMemoryStatusEx")

	pdh                              = windows.NewLazySystemDLL("pdh.dll")
	procPdhOpenQuery                 = pdh.NewProc("PdhOpenQueryW")
	procPdhAddEnglishCounter         = pdh.NewProc("PdhAddEnglishCounterW")
	procPdhCollectQueryData          = pdh.NewProc("PdhCollectQueryData")
	procPdhGetFormattedArray         = pdh.NewProc("PdhGetFormattedCounterArrayW")
	procPdhCloseQuery                = pdh.NewProc("PdhCloseQuery")
	dxgi                             = windows.NewLazySystemDLL("dxgi.dll")
	procCreateDXGIFactory1           = dxgi.NewProc("CreateDXGIFactory1")
	iidIDXGIFactory1                 = win.IID("770aae78-f26f-4dba-a829-253c83d1b387")
	errDXGINotFound          uintptr = 0x887A0002
)

// GPU is the adapter gpu/vram/gpuTemp come from (zero if none).
type GPU struct {
	Name      string
	Vendor    uint32
	LUID      string // as in counter instance names
	Dedicated uint64 // bytes of dedicated video memory
}

type Reader struct {
	mu                  sync.Mutex
	GPU                 GPU
	gpu                 gpuSource
	prevIdle, prevTotal uint64
}

func New() *Reader {
	r := &Reader{}
	if g, ok := pickAdapter(); ok {
		r.GPU = g
		r.gpu = &pdhGPU{g: g}
		if bin := findNvidiaSMI(); g.Vendor == 0x10de && bin != "" { // NVIDIA: also the temperature
			n := newNvidiaQuery("")
			n.bin = bin
			r.gpu = n
		}
		log.Printf("stats: GPU %s (vendor %04x, %d MiB)", g.Name, g.Vendor, g.Dedicated>>20)
	}
	return r
}

// findNvidiaSMI finds nvidia-smi: System32 with current drivers, NVSMI with older ones.
func findNvidiaSMI() string {
	if p, err := exec.LookPath("nvidia-smi"); err == nil {
		return p
	}
	p := filepath.Join(os.Getenv("ProgramFiles"), "NVIDIA Corporation", "NVSMI", "nvidia-smi.exe")
	if _, err := os.Stat(p); err == nil {
		return p
	}
	return ""
}

func filetime(ft windows.Filetime) uint64 {
	return uint64(ft.HighDateTime)<<32 | uint64(ft.LowDateTime)
}

// Sample returns the requested keys only. CPU load is relative to the previous call.
func (r *Reader) Sample(want map[string]bool) Sample {
	r.mu.Lock()
	defer r.mu.Unlock()
	s := Sample{}
	if want["cpu"] {
		var idle, kernel, user windows.Filetime
		if ok, _, _ := procGetSystemTimes.Call(uintptr(unsafe.Pointer(&idle)), uintptr(unsafe.Pointer(&kernel)), uintptr(unsafe.Pointer(&user))); ok != 0 {
			i, total := filetime(idle), filetime(kernel)+filetime(user) // kernel time includes idle
			if dt := total - r.prevTotal; r.prevTotal != 0 && dt > 0 && i >= r.prevIdle {
				s["cpu"] = int(100 * (dt - min(dt, i-r.prevIdle)) / dt)
			}
			r.prevIdle, r.prevTotal = i, total
		}
	}
	if want["ram"] {
		var m struct { // MEMORYSTATUSEX
			length, load               uint32
			totalPhys, availPhys, rest uint64
			_, _, _, _                 uint64
		}
		m.length = uint32(unsafe.Sizeof(m))
		if ok, _, _ := procGlobalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&m))); ok != 0 && m.totalPhys > 0 {
			s["ram"] = int(100 * (m.totalPhys - m.availPhys) / m.totalPhys)
		}
	}
	if r.gpu != nil && (want["gpu"] || want["vram"] || want["gpuTemp"]) {
		r.gpu.sample(want, s)
	}
	return s
}

// Stop ends background work (the nvidia-smi child, the counter query).
func (r *Reader) Stop() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.gpu != nil {
		r.gpu.stop()
	}
}

// pickAdapter enumerates DXGI adapters and keeps the hardware one with the
// most dedicated memory (a discrete card over an integrated one).
func pickAdapter() (GPU, bool) {
	var f win.Obj
	if r, _, _ := procCreateDXGIFactory1.Call(uintptr(unsafe.Pointer(iidIDXGIFactory1)), uintptr(unsafe.Pointer(&f))); win.HR(r) != nil || f == 0 {
		return GPU{}, false
	}
	defer f.Release()
	var best GPU
	for i := uintptr(0); ; i++ {
		var a win.Obj
		if r := f.Call(12, i, uintptr(unsafe.Pointer(&a))); r == errDXGINotFound || win.HR(r) != nil { // EnumAdapters1
			break
		}
		var d struct { // DXGI_ADAPTER_DESC1
			Description                          [128]uint16
			VendorID, DeviceID, SubSysID, Rev    uint32
			Dedicated, DedicatedSystem, SharedSy uintptr
			LUIDLow                              uint32
			LUIDHigh                             int32
			Flags                                uint32
		}
		err := a.CallErr(10, uintptr(unsafe.Pointer(&d))) // GetDesc1
		a.Release()
		if err != nil || d.Flags&2 != 0 || d.VendorID == 0x1414 { // software adapters (Basic Render Driver)
			continue
		}
		if uint64(d.Dedicated) > best.Dedicated || best.LUID == "" {
			best = GPU{Name: windows.UTF16ToString(d.Description[:]), Vendor: d.VendorID,
				LUID: luidString(d.LUIDHigh, d.LUIDLow), Dedicated: uint64(d.Dedicated)}
		}
	}
	return best, best.LUID != ""
}

// pdhGPU reads the GPU counters of one adapter. The query is opened by the
// first sample and closed by stop, so nothing runs while no gauge is shown.
type pdhGPU struct {
	g             GPU
	query         uintptr
	engine, local uintptr
}

func (p *pdhGPU) open() bool {
	if p.query != 0 {
		return true
	}
	if r, _, _ := procPdhOpenQuery.Call(0, 0, uintptr(unsafe.Pointer(&p.query))); r != 0 {
		p.query = 0
		return false
	}
	add := func(path string) uintptr {
		u, _ := windows.UTF16PtrFromString(path)
		var c uintptr
		if r, _, _ := procPdhAddEnglishCounter.Call(p.query, uintptr(unsafe.Pointer(u)), 0, uintptr(unsafe.Pointer(&c))); r != 0 {
			return 0
		}
		return c
	}
	p.engine = add(`\GPU Engine(*)\Utilization Percentage`)
	p.local = add(`\GPU Adapter Memory(*)\Dedicated Usage`)
	_, _, _ = procPdhCollectQueryData.Call(p.query) // rates need two collections
	return true
}

func (p *pdhGPU) sample(want map[string]bool, s Sample) {
	if !p.open() {
		return
	}
	if r, _, _ := procPdhCollectQueryData.Call(p.query); r != 0 {
		return
	}
	if want["gpu"] && p.engine != 0 {
		if v, ok := gpuLoad(pdhArray(p.engine), p.g.LUID); ok {
			s["gpu"] = v
		}
	}
	if want["vram"] && p.local != 0 && p.g.Dedicated > 0 {
		if used, ok := adapterMemory(pdhArray(p.local), p.g.LUID); ok {
			s["vram"] = int(min(100, 100*used/float64(p.g.Dedicated)))
		}
	}
}

func (p *pdhGPU) stop() {
	if p.query != 0 {
		_, _, _ = procPdhCloseQuery.Call(p.query)
		p.query, p.engine, p.local = 0, 0, 0
	}
}

// pdhArray reads every instance of a wildcard counter as doubles.
func pdhArray(counter uintptr) []pdhItem {
	const fmtDouble, moreData = 0x00000200, 0x800007D2
	var size, count uint32
	r, _, _ := procPdhGetFormattedArray.Call(counter, fmtDouble, uintptr(unsafe.Pointer(&size)), uintptr(unsafe.Pointer(&count)), 0)
	if r != moreData || size == 0 {
		return nil
	}
	buf := make([]byte, size)
	r, _, _ = procPdhGetFormattedArray.Call(counter, fmtDouble, uintptr(unsafe.Pointer(&size)), uintptr(unsafe.Pointer(&count)), uintptr(unsafe.Pointer(&buf[0])))
	if r != 0 {
		return nil
	}
	type item struct { // PDH_FMT_COUNTERVALUE_ITEM_W
		name   *uint16
		status uint32
		_      uint32
		value  float64
	}
	items := unsafe.Slice((*item)(unsafe.Pointer(&buf[0])), count)
	out := make([]pdhItem, 0, count)
	for _, it := range items {
		if it.status <= 1 && it.name != nil { // PDH_CSTATUS_VALID_DATA, NEW_DATA
			out = append(out, pdhItem{Name: windows.UTF16PtrToString(it.name), Value: it.value})
		}
	}
	return out
}
