package main

import (
	"bytes"
	_ "embed"
	"fmt"
	"image"
	"log"
	"os"
	"sync"
	"unsafe"

	"github.com/beedywool-vibecoded/roundbar/internal/daemon"
	"github.com/beedywool-vibecoded/roundbar/internal/win"
	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
	"golang.org/x/sys/windows"
)

// The tray app: the daemon plus a notification-area icon with a small menu.

//go:embed icon.svg
var iconSVG []byte

var (
	shell32                       = windows.NewLazySystemDLL("shell32.dll")
	procShellNotifyIconW          = shell32.NewProc("Shell_NotifyIconW")
	procCreatePopupMenu           = win.User32("CreatePopupMenu")
	procAppendMenuW               = win.User32("AppendMenuW")
	procSetMenuDefaultItem        = win.User32("SetMenuDefaultItem")
	procTrackPopupMenu            = win.User32("TrackPopupMenu")
	procDestroyMenu               = win.User32("DestroyMenu")
	procGetCursorPos              = win.User32("GetCursorPos")
	procSetForegroundWindow       = win.User32("SetForegroundWindow")
	procRegisterWindowMessageW    = win.User32("RegisterWindowMessageW")
	procGetSystemMetrics          = win.User32("GetSystemMetrics")
	procCreateIconIndirect        = win.User32("CreateIconIndirect")
	procSetProcessDpiAwarenessCtx = win.User32("SetProcessDpiAwarenessContext")
	gdi32                         = windows.NewLazySystemDLL("gdi32.dll")
	procCreateDIBSection          = gdi32.NewProc("CreateDIBSection")
	procCreateBitmap              = gdi32.NewProc("CreateBitmap")
	procDeleteObject              = gdi32.NewProc("DeleteObject")
)

const (
	nimAdd, nimModify, nimDelete = 0, 1, 2
	nifMessage, nifIcon, nifTip  = 1, 2, 4
	nifInfo                      = 0x10
	niifInfo                     = 1
	wmTray                       = win.WM_APP + 1
	wmLButtonUp                  = 0x0202
	wmRButtonUp                  = 0x0205
	wmContextMenu                = 0x007B
	mfString, mfGrayed           = 0, 1
	mfChecked, mfSeparator       = 8, 0x800
	tpmReturnCmd, tpmRightButton = 0x100, 2
	tpmNoNotify                  = 0x80

	cmdOpen, cmdAutostart, cmdQuit = 2, 3, 4
)

type notifyIconData struct {
	Size            uint32
	Wnd             uintptr
	ID              uint32
	Flags           uint32
	CallbackMessage uint32
	Icon            uintptr
	Tip             [128]uint16
	State           uint32
	StateMask       uint32
	Info            [256]uint16
	Version         uint32
	InfoTitle       [64]uint16
	InfoFlags       uint32
	GUIDItem        windows.GUID
	BalloonIcon     uintptr
}

type tray struct {
	d              *daemon.Daemon
	w              *win.Window
	icon           uintptr
	taskbarCreated uint32

	mu  sync.Mutex
	tip string
}

func runTray() error {
	_, _, _ = procSetProcessDpiAwarenessCtx.Call(^uintptr(3)) // PER_MONITOR_AWARE_V2: a sharp icon; fails quietly before 1703
	t := &tray{d: daemon.New()}
	name, _ := windows.UTF16PtrFromString("TaskbarCreated")
	r, _, _ := procRegisterWindowMessageW.Call(uintptr(unsafe.Pointer(name)))
	t.taskbarCreated = uint32(r)
	t.icon = makeIcon()
	w, err := win.NewWindow("RoundbarTray", true, t.wndProc, func(w *win.Window) error {
		t.w = w
		t.add()
		return nil
	})
	if err != nil {
		return err
	}
	t.w = w
	daemon.Notify = t.notify
	go t.follow()
	return t.d.Run() // blocks; Quit exits the process
}

func (t *tray) data() notifyIconData {
	nd := notifyIconData{Wnd: t.w.HWND, ID: 1}
	nd.Size = uint32(unsafe.Sizeof(nd))
	return nd
}

func copyUTF16(dst []uint16, s string) {
	u, _ := windows.UTF16FromString(s)
	if len(u) > len(dst) {
		u = append(u[:len(dst)-1], 0)
	}
	copy(dst, u)
}

// add puts the icon in the notification area (again, after Explorer restarts).
func (t *tray) add() {
	nd := t.data()
	nd.Flags = nifMessage | nifIcon | nifTip
	nd.CallbackMessage = wmTray
	nd.Icon = t.icon
	t.mu.Lock()
	tip := t.tip
	t.mu.Unlock()
	if tip == "" {
		tip = "Roundbar"
	}
	copyUTF16(nd.Tip[:], tip)
	_, _, _ = procShellNotifyIconW.Call(nimAdd, uintptr(unsafe.Pointer(&nd)))
}

func (t *tray) setTip(tip string) {
	t.mu.Lock()
	same := tip == t.tip
	t.tip = tip
	t.mu.Unlock()
	if same {
		return
	}
	nd := t.data()
	nd.Flags = nifTip
	copyUTF16(nd.Tip[:], tip)
	_, _, _ = procShellNotifyIconW.Call(nimModify, uintptr(unsafe.Pointer(&nd)))
}

// notify shows a balloon notification (a toast on Windows 10/11) from the icon.
func (t *tray) notify(title, body string) {
	nd := t.data()
	nd.Flags = nifInfo
	nd.InfoFlags = niifInfo
	copyUTF16(nd.InfoTitle[:], title)
	copyUTF16(nd.Info[:], body)
	_, _, _ = procShellNotifyIconW.Call(nimModify, uintptr(unsafe.Pointer(&nd)))
}

// follow keeps the tooltip on the device state.
func (t *tray) follow() {
	ch, _ := t.d.Watch()
	for {
		st := t.d.Status()
		t.setTip(statusLine(st))
		<-ch
	}
}

func statusLine(st daemon.Status) string {
	if st.Device != nil && st.Device.Connected {
		if st.Device.Port != "" {
			return fmt.Sprintf("Roundbar: connected (%s)", st.Device.Port)
		}
		return "Roundbar: connected"
	}
	return "Roundbar: not connected"
}

func (t *tray) wndProc(hwnd uintptr, msg uint32, wp, lp uintptr) (uintptr, bool) {
	switch {
	case msg == wmTray:
		switch uint32(lp & 0xffff) {
		case wmLButtonUp:
			go t.open()
		case wmRButtonUp, wmContextMenu:
			t.menu()
		}
		return 0, true
	case msg == t.taskbarCreated && msg != 0:
		t.add()
		return 0, true
	}
	return 0, false
}

func (t *tray) open() {
	if err := openConfigurator(); err != nil {
		log.Printf("configurator: %v", err)
	}
}

// menu runs on the window thread.
func (t *tray) menu() {
	m, _, _ := procCreatePopupMenu.Call()
	if m == 0 {
		return
	}
	defer procDestroyMenu.Call(m)
	item := func(flags, id uintptr, text string) {
		p, _ := windows.UTF16PtrFromString(text)
		_, _, _ = procAppendMenuW.Call(m, flags, id, uintptr(unsafe.Pointer(p)))
	}
	item(mfString|mfGrayed, 1, statusLine(t.d.Status()))
	item(mfSeparator, 0, "")
	item(mfString, cmdOpen, "Open configurator")
	auto := uintptr(mfString)
	if autostart() {
		auto |= mfChecked
	}
	item(auto, cmdAutostart, "Start at login")
	item(mfSeparator, 0, "")
	item(mfString, cmdQuit, "Quit")
	_, _, _ = procSetMenuDefaultItem.Call(m, cmdOpen, 0)
	var pt [2]int32
	_, _, _ = procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	// The menu only closes on an outside click if our window is in the foreground.
	_, _, _ = procSetForegroundWindow.Call(t.w.HWND)
	cmd, _, _ := procTrackPopupMenu.Call(m, tpmReturnCmd|tpmRightButton|tpmNoNotify, uintptr(pt[0]), uintptr(pt[1]), 0, t.w.HWND, 0)
	_ = win.PostMessage(t.w.HWND, 0, 0, 0) // WM_NULL, as the docs ask after TrackPopupMenu
	switch cmd {
	case cmdOpen:
		go t.open()
	case cmdAutostart:
		if err := setAutostart(!autostart()); err != nil {
			log.Printf("start at login: %v", err)
		}
	case cmdQuit:
		nd := t.data()
		_, _, _ = procShellNotifyIconW.Call(nimDelete, uintptr(unsafe.Pointer(&nd)))
		log.Printf("quit from the tray")
		os.Exit(0)
	}
}

// makeIcon renders the app icon (the configurator's favicon) at the size of
// small icons for this screen.
func makeIcon() uintptr {
	const smCxSmIcon = 49
	n, _, _ := procGetSystemMetrics.Call(smCxSmIcon)
	size := int(n)
	if size < 16 {
		size = 16
	}
	icon, err := oksvg.ReadIconStream(bytes.NewReader(iconSVG), oksvg.IgnoreErrorMode)
	if err != nil {
		return 0
	}
	icon.SetTarget(0, 0, float64(size), float64(size))
	rgba := image.NewRGBA(image.Rect(0, 0, size, size))
	icon.Draw(rasterx.NewDasher(size, size, rasterx.NewScannerGV(size, size, rgba, rgba.Bounds())), 1)

	type bitmapInfoHeader struct {
		size                  uint32
		width, height         int32
		planes, bitCount      uint16
		compression, sizeImg  uint32
		xPels, yPels          int32
		clrUsed, clrImportant uint32
	}
	bi := bitmapInfoHeader{width: int32(size), height: -int32(size), planes: 1, bitCount: 32}
	bi.size = uint32(unsafe.Sizeof(bi))
	var bits uintptr
	color, _, _ := procCreateDIBSection.Call(0, uintptr(unsafe.Pointer(&bi)), 0, uintptr(unsafe.Pointer(&bits)), 0, 0)
	if color == 0 || bits == 0 {
		return 0
	}
	defer procDeleteObject.Call(color)
	px := unsafe.Slice((*byte)(win.Ptr(bits)), size*size*4)
	for i := 0; i < size*size; i++ {
		r, g, b, a := int(rgba.Pix[4*i]), int(rgba.Pix[4*i+1]), int(rgba.Pix[4*i+2]), int(rgba.Pix[4*i+3])
		if a > 0 && a < 255 { // image.RGBA is premultiplied, icon bits are not
			r, g, b = r*255/a, g*255/a, b*255/a
		}
		px[4*i], px[4*i+1], px[4*i+2], px[4*i+3] = byte(b), byte(g), byte(r), byte(a)
	}
	mask, _, _ := procCreateBitmap.Call(uintptr(size), uintptr(size), 1, 1, 0)
	defer procDeleteObject.Call(mask)
	info := struct {
		isIcon             int32
		xHotspot, yHotspot uint32
		mask, color        uintptr
	}{isIcon: 1, mask: mask, color: color}
	h, _, _ := procCreateIconIndirect.Call(uintptr(unsafe.Pointer(&info)))
	return h
}
