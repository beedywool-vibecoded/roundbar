package win

import (
	"errors"
	"runtime"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	user32   = windows.NewLazySystemDLL("user32.dll")
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")

	procRegisterClassExW = user32.NewProc("RegisterClassExW")
	procCreateWindowExW  = user32.NewProc("CreateWindowExW")
	procDefWindowProcW   = user32.NewProc("DefWindowProcW")
	procDestroyWindow    = user32.NewProc("DestroyWindow")
	procGetMessageW      = user32.NewProc("GetMessageW")
	procTranslateMessage = user32.NewProc("TranslateMessage")
	procDispatchMessageW = user32.NewProc("DispatchMessageW")
	procPostMessageW     = user32.NewProc("PostMessageW")
	procPostQuitMessage  = user32.NewProc("PostQuitMessage")
	procGetModuleHandleW = kernel32.NewProc("GetModuleHandleW")
)

// User32 and Kernel32 give other packages lazy procs from the same DLLs.
func User32(name string) *windows.LazyProc   { return user32.NewProc(name) }
func Kernel32(name string) *windows.LazyProc { return kernel32.NewProc(name) }

const (
	WM_DESTROY = 0x0002
	WM_CLOSE   = 0x0010
	WM_APP     = 0x8000
	wmRun      = WM_APP + 0x3f0 // run queued funcs on the window's thread
)

// HWND_MESSAGE as the parent makes a message-only window.
const HWND_MESSAGE = ^uintptr(2) // (HWND)-3

type wndclassex struct {
	size       uint32
	style      uint32
	wndProc    uintptr
	clsExtra   int32
	wndExtra   int32
	instance   uintptr
	icon       uintptr
	cursor     uintptr
	background uintptr
	menuName   *uint16
	className  *uint16
	iconSm     uintptr
}

type msg struct {
	hwnd    uintptr
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      [2]int32
	private uint32
}

// WndProc handles a message; return handled=false to let Windows do its default.
type WndProc func(hwnd uintptr, msg uint32, wParam, lParam uintptr) (result uintptr, handled bool)

// Window is a hidden window on its own locked OS thread, pumping messages.
// Win events, clipboard listeners, tray icons and shell hooks deliver there.
type Window struct {
	HWND uintptr
	proc WndProc
	mu   sync.Mutex
	runq []func()
}

var (
	windowsMu  sync.Mutex
	byHWND     = map[uintptr]*Window{}
	createMu   sync.Mutex // one CreateWindowEx at a time, so `creating` is unambiguous
	creating   *Window    // the window whose CreateWindowEx is running (messages before it returns)
	wndProcCB  uintptr
	registered = map[string]bool{}
)

func wndProc(hwnd, m, wp, lp uintptr) uintptr {
	windowsMu.Lock()
	w := byHWND[hwnd]
	if w == nil {
		w = creating
	}
	windowsMu.Unlock()
	if w != nil {
		if m == wmRun {
			w.mu.Lock()
			q := w.runq
			w.runq = nil
			w.mu.Unlock()
			for _, f := range q {
				f()
			}
			return 0
		}
		if w.proc != nil {
			if r, ok := w.proc(hwnd, uint32(m), wp, lp); ok {
				return r
			}
		}
	}
	r, _, _ := procDefWindowProcW.Call(hwnd, m, wp, lp)
	return r
}

// NewWindow starts a thread, creates a window of a private class there
// (message-only unless topLevel: shell hooks and broadcasts need a top-level
// one, which stays hidden), runs setup on that thread, then pumps messages
// until the window is destroyed. proc sees every message.
func NewWindow(class string, topLevel bool, proc WndProc, setup func(w *Window) error) (*Window, error) {
	w := &Window{proc: proc}
	ready := make(chan error, 1)
	go func() {
		runtime.LockOSThread() // the window and its hooks belong to this thread for good
		hwnd, err := w.create(class, topLevel)
		if err != nil {
			ready <- err
			return
		}
		if setup != nil {
			if err := setup(w); err != nil {
				_, _, _ = procDestroyWindow.Call(hwnd)
				ready <- err
				return
			}
		}
		ready <- nil
		var m msg
		for {
			r, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
			if int32(r) <= 0 {
				return
			}
			_, _, _ = procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
			_, _, _ = procDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
		}
	}()
	if err := <-ready; err != nil {
		return nil, err
	}
	return w, nil
}

func (w *Window) create(class string, topLevel bool) (uintptr, error) {
	createMu.Lock()
	defer createMu.Unlock()
	windowsMu.Lock()
	defer windowsMu.Unlock()
	if wndProcCB == 0 {
		wndProcCB = syscall.NewCallback(wndProc)
	}
	inst, _, _ := procGetModuleHandleW.Call(0)
	cls, _ := windows.UTF16PtrFromString(class)
	if !registered[class] {
		wc := wndclassex{wndProc: wndProcCB, instance: inst, className: cls}
		wc.size = uint32(unsafe.Sizeof(wc))
		if r, _, err := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); r == 0 {
			return 0, err
		}
		registered[class] = true
	}
	parent := HWND_MESSAGE
	var exStyle uintptr
	if topLevel {
		parent = 0
		exStyle = 0x00000080 // WS_EX_TOOLWINDOW: never in the taskbar or Alt+Tab
	}
	creating = w
	windowsMu.Unlock()
	hwnd, _, err := procCreateWindowExW.Call(exStyle, uintptr(unsafe.Pointer(cls)), uintptr(unsafe.Pointer(cls)), 0, 0, 0, 0, 0, parent, 0, inst, 0)
	windowsMu.Lock()
	creating = nil
	if hwnd == 0 {
		return 0, err
	}
	w.HWND = hwnd
	byHWND[hwnd] = w
	return hwnd, nil
}

// Post queues f to run on the window's thread.
func (w *Window) Post(f func()) error {
	w.mu.Lock()
	w.runq = append(w.runq, f)
	w.mu.Unlock()
	if r, _, err := procPostMessageW.Call(w.HWND, wmRun, 0, 0); r == 0 {
		return err
	}
	return nil
}

// Do runs f on the window's thread and waits for it.
func (w *Window) Do(f func()) error {
	done := make(chan struct{})
	if err := w.Post(func() { f(); close(done) }); err != nil {
		return err
	}
	<-done
	return nil
}

// Destroy ends the window and its message loop.
func (w *Window) Destroy() {
	_ = w.Post(func() {
		_, _, _ = procDestroyWindow.Call(w.HWND)
		windowsMu.Lock()
		delete(byHWND, w.HWND)
		windowsMu.Unlock()
		_, _, _ = procPostQuitMessage.Call(0)
	})
}

// PostMessage posts a message to any window.
func PostMessage(hwnd uintptr, m uint32, wp, lp uintptr) error {
	if r, _, err := procPostMessageW.Call(hwnd, uintptr(m), wp, lp); r == 0 {
		return err
	}
	return nil
}

var ErrNoWindow = errors.New("no window")
