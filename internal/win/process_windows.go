package win

import (
	"path/filepath"
	"strings"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	version                     = windows.NewLazySystemDLL("version.dll")
	procGetFileVersionInfoSizeW = version.NewProc("GetFileVersionInfoSizeW")
	procGetFileVersionInfoW     = version.NewProc("GetFileVersionInfoW")
	procVerQueryValueW          = version.NewProc("VerQueryValueW")
)

// ExePath is the full image path of a process ("" if it can't be opened).
func ExePath(pid uint32) string {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return ""
	}
	defer windows.CloseHandle(h)
	buf := make([]uint16, windows.MAX_LONG_PATH)
	n := uint32(len(buf))
	if err := windows.QueryFullProcessImageName(h, 0, &buf[0], &n); err != nil {
		return ""
	}
	return windows.UTF16ToString(buf[:n])
}

// ExeID is the lowercase file name without ".exe": the app id roundbar uses on Windows.
func ExeID(path string) string {
	b := strings.ToLower(filepath.Base(path))
	return strings.TrimSuffix(b, ".exe")
}

var descCache sync.Map // exe path -> description

// FileDescription reads the FileDescription of an executable's version
// resource ("Mozilla Firefox", "Visual Studio Code"), cached per path.
func FileDescription(path string) string {
	if path == "" {
		return ""
	}
	if v, ok := descCache.Load(path); ok {
		return v.(string)
	}
	d := fileDescription(path)
	descCache.Store(path, d)
	return d
}

func fileDescription(path string) string {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return ""
	}
	size, _, _ := procGetFileVersionInfoSizeW.Call(uintptr(unsafe.Pointer(p)), 0)
	if size == 0 {
		return ""
	}
	data := make([]byte, size)
	if r, _, _ := procGetFileVersionInfoW.Call(uintptr(unsafe.Pointer(p)), 0, size, uintptr(unsafe.Pointer(&data[0]))); r == 0 {
		return ""
	}
	query := func(q string) (uintptr, uint32) {
		qp, _ := windows.UTF16PtrFromString(q)
		var out uintptr
		var n uint32
		if r, _, _ := procVerQueryValueW.Call(uintptr(unsafe.Pointer(&data[0])), uintptr(unsafe.Pointer(qp)), uintptr(unsafe.Pointer(&out)), uintptr(unsafe.Pointer(&n))); r == 0 {
			return 0, 0
		}
		return out, n
	}
	langs := []string{"040904b0", "040904e4", "000004b0"}
	if t, n := query(`\VarFileInfo\Translation`); t != 0 && n >= 4 {
		tr := unsafe.Slice((*uint16)(Ptr(t)), n/2)
		for i := 0; i+1 < len(tr); i += 2 {
			langs = append([]string{hex4(tr[i]) + hex4(tr[i+1])}, langs...)
		}
	}
	for _, l := range langs {
		if s, n := query(`\StringFileInfo\` + l + `\FileDescription`); s != 0 && n > 0 {
			if d := strings.TrimSpace(windows.UTF16ToString(unsafe.Slice((*uint16)(Ptr(s)), n))); d != "" {
				return d
			}
		}
	}
	return ""
}

func hex4(v uint16) string {
	const digits = "0123456789abcdef"
	return string([]byte{digits[v>>12], digits[v>>8&15], digits[v>>4&15], digits[v&15]})
}

// Process is one entry of a process snapshot.
type Process struct {
	PID, Parent uint32
	Exe         string // file name only
}

// Processes lists running processes.
func Processes() []Process {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil
	}
	defer windows.CloseHandle(snap)
	var e windows.ProcessEntry32
	e.Size = uint32(unsafe.Sizeof(e))
	var out []Process
	for err = windows.Process32First(snap, &e); err == nil; err = windows.Process32Next(snap, &e) {
		out = append(out, Process{PID: e.ProcessID, Parent: e.ParentProcessID, Exe: windows.UTF16ToString(e.ExeFile[:])})
	}
	return out
}
