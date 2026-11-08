package desktop

import (
	"errors"
	"os"
	"strconv"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/beedywool-vibecoded/roundbar/internal/icons"
	"github.com/beedywool-vibecoded/roundbar/internal/steam"
	"github.com/beedywool-vibecoded/roundbar/internal/win"
	"golang.org/x/sys/windows"
)

// winShell follows the Windows desktop from one hidden window on its own
// thread: SetWinEventHook(EVENT_SYSTEM_FOREGROUND) for the focused app, a
// shell hook (RegisterShellHookWindow: window created / destroyed / retitled,
// what the taskbar hears) for the window list, AddClipboardFormatListener for
// the clipboard history. Windows posts all of it to that thread's message
// loop; nothing polls. Other apps' notifications are out of reach: Windows
// only shows them to packaged apps (UserNotificationListener needs package
// identity), so Caps.Notifications is false.
type winShell struct {
	none
	ev            events
	w             *win.Window
	hook, objHook uintptr
	shellMsg      uint32
	cfExclude     uint32 // registered clipboard formats we look for
	cfViewerIgn   uint32
	cfCanInclude  uint32

	mu       sync.Mutex
	clips    []string
	focus    Focus
	lastHKL  uint32
	debounce *time.Timer
}

var (
	procSetWinEventHook            = win.User32("SetWinEventHook")
	procUnhookWinEvent             = win.User32("UnhookWinEvent")
	procGetForegroundWindow        = win.User32("GetForegroundWindow")
	procGetWindowThreadProcessId   = win.User32("GetWindowThreadProcessId")
	procGetWindowTextW             = win.User32("GetWindowTextW")
	procGetClassNameW              = win.User32("GetClassNameW")
	procIsWindowVisible            = win.User32("IsWindowVisible")
	procIsWindow                   = win.User32("IsWindow")
	procGetWindow                  = win.User32("GetWindow")
	procGetWindowLongPtrW          = win.User32("GetWindowLongPtrW")
	procEnumWindows                = win.User32("EnumWindows")
	procEnumChildWindows           = win.User32("EnumChildWindows")
	procGetAncestor                = win.User32("GetAncestor")
	procIsIconic                   = win.User32("IsIconic")
	procShowWindow                 = win.User32("ShowWindow")
	procSetForegroundWindow        = win.User32("SetForegroundWindow")
	procBringWindowToTop           = win.User32("BringWindowToTop")
	procAttachThreadInput          = win.User32("AttachThreadInput")
	procKeybdEvent                 = win.User32("keybd_event")
	procRegisterShellHookWindow    = win.User32("RegisterShellHookWindow")
	procDeregisterShellHookWindow  = win.User32("DeregisterShellHookWindow")
	procRegisterWindowMessageW     = win.User32("RegisterWindowMessageW")
	procAddClipboardFormatListener = win.User32("AddClipboardFormatListener")
	procRemoveClipboardFormatList  = win.User32("RemoveClipboardFormatListener")
	procOpenClipboard              = win.User32("OpenClipboard")
	procCloseClipboard             = win.User32("CloseClipboard")
	procEmptyClipboard             = win.User32("EmptyClipboard")
	procGetClipboardData           = win.User32("GetClipboardData")
	procSetClipboardData           = win.User32("SetClipboardData")
	procEnumClipboardFormats       = win.User32("EnumClipboardFormats")
	procRegisterClipboardFormatW   = win.User32("RegisterClipboardFormatW")
	procGetKeyboardLayout          = win.User32("GetKeyboardLayout")
	procGlobalAlloc                = win.Kernel32("GlobalAlloc")
	procGlobalFree                 = win.Kernel32("GlobalFree")
	procGlobalLock                 = win.Kernel32("GlobalLock")
	procGlobalUnlock               = win.Kernel32("GlobalUnlock")
	procGlobalSize                 = win.Kernel32("GlobalSize")
	procGetCurrentThreadId         = win.Kernel32("GetCurrentThreadId")
	dwmapi                         = windows.NewLazySystemDLL("dwmapi.dll")
	procDwmGetWindowAttribute      = dwmapi.NewProc("DwmGetWindowAttribute")
)

const (
	eventSystemForeground = 0x0003
	eventObjectCreate     = 0x8000
	eventObjectDestroy    = 0x8001
	eventObjectShow       = 0x8002
	eventObjectHide       = 0x8003
	eventObjectNameChange = 0x800C
	winEventOutOfContext  = 0x0000
	winEventSkipOwnProc   = 0x0002
	wmClipboardUpdate     = 0x031D
	cfUnicodeText         = 13
	gwOwner               = 4
	gwlExStyle            = ^uintptr(19) // -20
	gaRoot                = 2
	dwmwaCloaked          = 14
	swRestore             = 9
)

var (
	winEventCB   uintptr
	winEventOnce sync.Once
	theShell     *winShell // the WinEvent callback has no context argument; there is one shell
)

func newWinShell(ev events) (*winShell, error) {
	s := &winShell{ev: ev}
	winEventOnce.Do(func() {
		winEventCB = syscall.NewCallback(func(hook, event, hwnd, idObject, idChild, thread, time uintptr) uintptr {
			if theShell != nil {
				theShell.winEvent(uint32(event), hwnd, int32(idObject), int32(idChild))
			}
			return 0
		})
	})
	theShell = s
	reg := func(name string) uint32 {
		p, _ := windows.UTF16PtrFromString(name)
		r, _, _ := procRegisterClipboardFormatW.Call(uintptr(unsafe.Pointer(p)))
		return uint32(r)
	}
	s.cfExclude = reg("ExcludeClipboardContentFromMonitorProcessing")
	s.cfViewerIgn = reg("Clipboard Viewer Ignore")
	s.cfCanInclude = reg("CanIncludeInClipboardHistory")
	shellHook, _ := windows.UTF16PtrFromString("SHELLHOOK")
	r, _, _ := procRegisterWindowMessageW.Call(uintptr(unsafe.Pointer(shellHook)))
	s.shellMsg = uint32(r)

	w, err := win.NewWindow("RoundbarShell", true, s.wndProc, func(w *win.Window) error {
		s.w = w
		h, _, err := procSetWinEventHook.Call(eventSystemForeground, eventSystemForeground, 0, winEventCB, 0, 0, winEventOutOfContext|winEventSkipOwnProc)
		if h == 0 {
			return err
		}
		s.hook = h
		if ok, _, _ := procRegisterShellHookWindow.Call(w.HWND); ok == 0 {
			// No shell hook (an unusual shell): follow top-level windows through WinEvents.
			s.objHook, _, _ = procSetWinEventHook.Call(eventObjectCreate, eventObjectNameChange, 0, winEventCB, 0, 0, winEventOutOfContext|winEventSkipOwnProc)
		}
		_, _, _ = procAddClipboardFormatListener.Call(w.HWND)
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.w = w
	if f, ok := s.focusOf(foreground()); ok {
		s.mu.Lock()
		s.focus = f
		s.mu.Unlock()
	}
	return s, nil
}

func (s *winShell) Caps() Caps { return Caps{Focus: true, Windows: true, Clipboard: true} }

func (s *winShell) Close() {
	_ = s.w.Do(func() {
		_, _, _ = procUnhookWinEvent.Call(s.hook)
		if s.objHook != 0 {
			_, _, _ = procUnhookWinEvent.Call(s.objHook)
		}
		_, _, _ = procDeregisterShellHookWindow.Call(s.w.HWND)
		_, _, _ = procRemoveClipboardFormatList.Call(s.w.HWND)
	})
	s.w.Destroy()
}

func foreground() uintptr {
	h, _, _ := procGetForegroundWindow.Call()
	return h
}

// winEvent runs on the shell thread.
func (s *winShell) winEvent(event uint32, hwnd uintptr, idObject, idChild int32) {
	switch event {
	case eventSystemForeground:
		if f, ok := s.focusOf(hwnd); ok {
			s.mu.Lock()
			s.focus = f
			s.mu.Unlock()
			s.ev.focus(f)
		}
		s.windowsChanged() // most recent first
		s.checkLayout(hwnd)
	case eventObjectCreate, eventObjectDestroy, eventObjectShow, eventObjectHide, eventObjectNameChange:
		if idObject == 0 && idChild == 0 && isTopLevel(hwnd) { // OBJID_WINDOW, CHILDID_SELF
			s.windowsChanged()
		}
	}
}

func isTopLevel(hwnd uintptr) bool {
	root, _, _ := procGetAncestor.Call(hwnd, gaRoot)
	return root == hwnd
}

func (s *winShell) wndProc(hwnd uintptr, msg uint32, wp, lp uintptr) (uintptr, bool) {
	switch {
	case msg == s.shellMsg && s.shellMsg != 0:
		switch wp &^ 0x8000 { // HSHELL_RUDEAPPACTIVATED etc. carry the high bit
		case 1, 2, 6: // HSHELL_WINDOWCREATED, WINDOWDESTROYED, REDRAW (title change)
			s.windowsChanged()
		}
		return 0, true
	case msg == wmClipboardUpdate:
		s.readClipboard()
		return 0, true
	}
	return 0, false
}

// windowsChanged tells the daemon once things settle (titles change in bursts).
func (s *winShell) windowsChanged() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.debounce != nil {
		s.debounce.Stop()
	}
	s.debounce = time.AfterFunc(150*time.Millisecond, s.ev.windows)
}

func windowText(hwnd uintptr) string {
	buf := make([]uint16, 512)
	n, _, _ := procGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	return windows.UTF16ToString(buf[:n])
}

func className(hwnd uintptr) string {
	buf := make([]uint16, 256)
	n, _, _ := procGetClassNameW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	return windows.UTF16ToString(buf[:n])
}

func windowPID(hwnd uintptr) (pid, tid uint32) {
	r, _, _ := procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	return pid, uint32(r)
}

var childScan struct {
	sync.Mutex
	cb       uintptr
	framePID uint32
	found    uint32
}

// appPID is the process behind a window. UWP apps live in a frame owned by
// ApplicationFrameHost.exe; the app is the process of a child window.
func appPID(hwnd uintptr) uint32 {
	pid, _ := windowPID(hwnd)
	if win.ExeID(win.ExePath(pid)) != "applicationframehost" {
		return pid
	}
	childScan.Lock()
	defer childScan.Unlock()
	if childScan.cb == 0 {
		childScan.cb = syscall.NewCallback(func(child, _ uintptr) uintptr {
			if p, _ := windowPID(child); p != childScan.framePID {
				childScan.found = p
				return 0
			}
			return 1
		})
	}
	childScan.framePID, childScan.found = pid, 0
	_, _, _ = procEnumChildWindows.Call(hwnd, childScan.cb, 0)
	if childScan.found != 0 {
		return childScan.found
	}
	return pid
}

// focusOf builds the Focus for a foreground window; ok=false for the shell
// itself (desktop, taskbar, task switcher), which keeps the current profile.
func (s *winShell) focusOf(hwnd uintptr) (Focus, bool) {
	if hwnd == 0 {
		return Focus{}, false
	}
	exe := win.ExePath(appPID(hwnd))
	if exe == "" || ignoredFocus(className(hwnd), win.ExeID(exe)) {
		return Focus{}, false
	}
	return winFocus(exe, win.FileDescription(exe), windowText(hwnd), steam.AppForExe(exe)), true
}

func (s *winShell) Focus() (Focus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.focus, nil
}

// checkLayout: the keyboard layout is per thread on Windows, so the layout to
// type with can change with the focus (and is re-read then: switching
// layouts inside one window is noticed on the next focus change).
func (s *winShell) checkLayout(hwnd uintptr) {
	hkl := hklOf(hwnd)
	s.mu.Lock()
	changed := hkl != 0 && hkl != s.lastHKL
	if changed {
		s.lastHKL = hkl
	}
	s.mu.Unlock()
	if changed {
		if d, ok := s.ev.(*Desktop); ok {
			kick(d.LayoutChanged)
		}
	}
}

func hklOf(hwnd uintptr) uint32 {
	if hwnd == 0 {
		return 0
	}
	_, tid := windowPID(hwnd)
	r, _, _ := procGetKeyboardLayout.Call(uintptr(tid))
	return uint32(r)
}

func (s *winShell) foregroundHKL() uint32 { return hklOf(foreground()) }

// ---- window list --------------------------------------------------------------

var enumWin struct {
	sync.Mutex
	cb   uintptr
	list []uintptr
}

func topLevelWindows() []uintptr {
	enumWin.Lock()
	defer enumWin.Unlock()
	if enumWin.cb == 0 {
		enumWin.cb = syscall.NewCallback(func(hwnd, _ uintptr) uintptr {
			enumWin.list = append(enumWin.list, hwnd)
			return 1
		})
	}
	enumWin.list = nil
	_, _, _ = procEnumWindows.Call(enumWin.cb, 0)
	return append([]uintptr(nil), enumWin.list...)
}

func infoOf(hwnd uintptr) winInfo {
	vis, _, _ := procIsWindowVisible.Call(hwnd)
	owner, _, _ := procGetWindow.Call(hwnd, gwOwner)
	ex, _, _ := procGetWindowLongPtrW.Call(hwnd, gwlExStyle)
	var cloaked uint32
	_, _, _ = procDwmGetWindowAttribute.Call(hwnd, dwmwaCloaked, uintptr(unsafe.Pointer(&cloaked)), 4)
	return winInfo{Visible: vis != 0, Cloaked: cloaked != 0, Owned: owner != 0, ExStyle: uint32(ex),
		Title: windowText(hwnd), Class: className(hwnd)}
}

// Windows lists app windows in Z order, which is most recently used first.
func (s *winShell) Windows() ([]Window, error) {
	self := uint32(os.Getpid())
	var out []Window
	for _, h := range topLevelWindows() {
		in := infoOf(h)
		if !isAppWindow(in) {
			continue
		}
		pid := appPID(h)
		if pid == self {
			continue
		}
		exe := win.ExePath(pid)
		id := win.ExeID(exe)
		w := Window{ID: strconv.FormatUint(uint64(h), 10), App: id, Title: in.Title}
		if exe != "" {
			w.Icon = "exe:" + id
			icons.Alias(w.Icon, exe)
		}
		out = append(out, w)
	}
	return out, nil
}

// Activate brings a window forward. Windows only lets the process that got
// the last input take the foreground, and the input comes from the device,
// so attach to the foreground thread's input first (the usual workaround),
// and as a last resort tap Alt, which unlocks SetForegroundWindow.
func (s *winShell) Activate(id string) error {
	n, err := strconv.ParseUint(id, 10, 64)
	if err != nil {
		return err
	}
	h := uintptr(n)
	if ok, _, _ := procIsWindow.Call(h); ok == 0 {
		return errors.New("no such window")
	}
	var res error
	_ = s.w.Do(func() {
		if ic, _, _ := procIsIconic.Call(h); ic != 0 {
			_, _, _ = procShowWindow.Call(h, swRestore)
		}
		if setForeground(h) {
			return
		}
		self, _, _ := procGetCurrentThreadId.Call()
		_, fgThread := windowPID(foreground())
		if fgThread != 0 && uintptr(fgThread) != self {
			_, _, _ = procAttachThreadInput.Call(self, uintptr(fgThread), 1)
			ok := setForeground(h)
			_, _, _ = procAttachThreadInput.Call(self, uintptr(fgThread), 0)
			if ok {
				return
			}
		}
		const vkMenu, keyUp = 0x12, 0x0002
		_, _, _ = procKeybdEvent.Call(vkMenu, 0, 0, 0)
		_, _, _ = procKeybdEvent.Call(vkMenu, 0, keyUp, 0)
		if !setForeground(h) {
			res = errors.New("Windows refused to bring the window forward")
		}
	})
	return res
}

func setForeground(h uintptr) bool {
	_, _, _ = procSetForegroundWindow.Call(h)
	_, _, _ = procBringWindowToTop.Call(h)
	return foreground() == h
}

// ---- clipboard ----------------------------------------------------------------

// openClipboard retries: whoever just wrote may still hold it.
func (s *winShell) openClipboard() bool {
	for i := 0; i < 10; i++ {
		if ok, _, _ := procOpenClipboard.Call(s.w.HWND); ok != 0 {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}

func globalBytes(h uintptr) []byte {
	if h == 0 {
		return nil
	}
	p, _, _ := procGlobalLock.Call(h)
	if p == 0 {
		return nil
	}
	defer procGlobalUnlock.Call(h)
	n, _, _ := procGlobalSize.Call(h)
	return append([]byte(nil), unsafe.Slice((*byte)(win.Ptr(p)), n)...)
}

// readClipboard runs on the shell thread for WM_CLIPBOARDUPDATE.
func (s *winShell) readClipboard() {
	if !s.openClipboard() {
		return
	}
	present := map[string]bool{}
	var canInclude []byte
	hasText := false
	for f, _, _ := procEnumClipboardFormats.Call(0); f != 0; f, _, _ = procEnumClipboardFormats.Call(f) {
		switch uint32(f) {
		case cfUnicodeText:
			hasText = true
		case s.cfExclude:
			present["ExcludeClipboardContentFromMonitorProcessing"] = true
		case s.cfViewerIgn:
			present["Clipboard Viewer Ignore"] = true
		case s.cfCanInclude:
			present["CanIncludeInClipboardHistory"] = true
			h, _, _ := procGetClipboardData.Call(f)
			canInclude = globalBytes(h)
		}
	}
	text := ""
	if hasText && !clipboardPrivate(present, canInclude) {
		h, _, _ := procGetClipboardData.Call(cfUnicodeText)
		if b := globalBytes(h); len(b) >= 2 {
			text = windows.UTF16ToString(unsafe.Slice((*uint16)(unsafe.Pointer(&b[0])), len(b)/2))
		}
	}
	_, _, _ = procCloseClipboard.Call()
	if text == "" {
		return
	}
	s.mu.Lock()
	var ok bool
	s.clips, ok = pushHistory(s.clips, text)
	s.mu.Unlock()
	if ok {
		s.ev.clips()
	}
}

func (s *winShell) Clipboard() ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.clips...), nil
}

func (s *winShell) SetClipboard(text string) error {
	u, err := windows.UTF16FromString(text)
	if err != nil {
		return err
	}
	var res error
	_ = s.w.Do(func() {
		if !s.openClipboard() {
			res = errors.New("the clipboard is busy")
			return
		}
		defer procCloseClipboard.Call()
		_, _, _ = procEmptyClipboard.Call()
		const gmemMoveable = 0x0002
		size := uintptr(len(u) * 2)
		h, _, err := procGlobalAlloc.Call(gmemMoveable, size)
		if h == 0 {
			res = err
			return
		}
		p, _, _ := procGlobalLock.Call(h)
		if p == 0 {
			_, _, _ = procGlobalFree.Call(h)
			res = errors.New("GlobalLock failed")
			return
		}
		copy(unsafe.Slice((*uint16)(win.Ptr(p)), len(u)), u)
		_, _, _ = procGlobalUnlock.Call(h)
		if r, _, err := procSetClipboardData.Call(cfUnicodeText, h); r == 0 {
			_, _, _ = procGlobalFree.Call(h) // still ours when SetClipboardData fails
			res = err
		}
	})
	return res
}
