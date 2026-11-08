package steam

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/beedywool-vibecoded/roundbar/internal/win"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// root is Steam's install folder (HKCU\Software\Valve\Steam SteamPath).
func root() string {
	if k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Valve\Steam`, registry.QUERY_VALUE); err == nil {
		defer k.Close()
		if p, _, err := k.GetStringValue("SteamPath"); err == nil && p != "" {
			return filepath.Clean(p)
		}
	}
	return `C:\Program Files (x86)\Steam`
}

// GamePIDs lists the processes of a game: those whose image lives in its
// install folder, plus everything they started.
func GamePIDs(appid string) []int {
	dir := InstallPath(appid)
	if dir == "" {
		return nil
	}
	prefix := strings.ToLower(filepath.Clean(dir)) + `\`
	procs := win.Processes()
	in := map[uint32]bool{}
	for _, p := range procs {
		if exe := strings.ToLower(win.ExePath(p.PID)); exe != "" && strings.HasPrefix(exe, prefix) {
			in[p.PID] = true
		}
	}
	for changed := true; changed; { // children of game processes (crash handlers, helpers)
		changed = false
		for _, p := range procs {
			if !in[p.PID] && in[p.Parent] && p.PID != 0 {
				in[p.PID], changed = true, true
			}
		}
	}
	var out []int
	for pid := range in {
		out = append(out, int(pid))
	}
	return out
}

var closeWindows struct {
	sync.Mutex
	cb   uintptr
	want map[uint32]bool
}

var (
	procEnumWindows              = win.User32("EnumWindows")
	procGetWindowThreadProcessId = win.User32("GetWindowThreadProcessId")
)

// Close ends a game: WM_CLOSE to its windows (the polite way, games save and
// quit), then TerminateProcess for whatever is left after 6 s.
func Close(appid string) (int, error) {
	pids := GamePIDs(appid)
	if len(pids) == 0 {
		return 0, fmt.Errorf("no running processes for app %s", appid)
	}
	closeWindows.Lock()
	closeWindows.want = map[uint32]bool{}
	for _, p := range pids {
		closeWindows.want[uint32(p)] = true
	}
	if closeWindows.cb == 0 {
		closeWindows.cb = syscall.NewCallback(func(hwnd, _ uintptr) uintptr {
			var pid uint32
			_, _, _ = procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
			if closeWindows.want[pid] {
				_ = win.PostMessage(hwnd, win.WM_CLOSE, 0, 0)
			}
			return 1
		})
	}
	_, _, _ = procEnumWindows.Call(closeWindows.cb, 0)
	closeWindows.Unlock()
	go func() {
		for i := 0; i < 20; i++ {
			time.Sleep(300 * time.Millisecond)
			if len(GamePIDs(appid)) == 0 {
				return
			}
		}
		for _, p := range GamePIDs(appid) {
			if h, err := windows.OpenProcess(windows.PROCESS_TERMINATE, false, uint32(p)); err == nil {
				_ = windows.TerminateProcess(h, 1)
				windows.CloseHandle(h)
			}
		}
	}()
	return len(pids), nil
}
