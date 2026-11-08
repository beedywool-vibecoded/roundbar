package device

import (
	"os"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// Ports lists the COM ports ("COM5") of every attached USB device with these
// ids: Enum\USB\VID_xxxx&PID_yyyy[&MI_nn]\<instance>\Device Parameters\PortName,
// kept only if SERIALCOMM lists the port (the device is present now).
func Ports(vid, pid uint16) []string {
	present := map[string]bool{}
	if k, err := registry.OpenKey(registry.LOCAL_MACHINE, `HARDWARE\DEVICEMAP\SERIALCOMM`, registry.QUERY_VALUE); err == nil {
		names, _ := k.ReadValueNames(0)
		for _, n := range names {
			if v, _, err := k.GetStringValue(n); err == nil {
				present[strings.ToUpper(v)] = true
			}
		}
		k.Close()
	}
	usb, err := registry.OpenKey(registry.LOCAL_MACHINE, `SYSTEM\CurrentControlSet\Enum\USB`, registry.ENUMERATE_SUB_KEYS)
	if err != nil {
		return nil
	}
	defer usb.Close()
	keys, _ := usb.ReadSubKeyNames(0)
	sort.Strings(keys) // MI_00 (the CDC data interface comes first) before others
	var out []string
	for _, key := range keys {
		if !usbKeyMatches(key, vid, pid) {
			continue
		}
		dev, err := registry.OpenKey(usb, key, registry.ENUMERATE_SUB_KEYS)
		if err != nil {
			continue
		}
		insts, _ := dev.ReadSubKeyNames(0)
		for _, inst := range insts {
			if p, err := registry.OpenKey(dev, inst+`\Device Parameters`, registry.QUERY_VALUE); err == nil {
				if name, _, err := p.GetStringValue("PortName"); err == nil && present[strings.ToUpper(name)] {
					out = append(out, name)
				}
				p.Close()
			}
		}
		dev.Close()
	}
	return out
}

// Find returns the COM port of an attached roundbar ("COM5"), or "".
func Find() string {
	if p := Ports(VID, PID); len(p) > 0 {
		return p[0]
	}
	return ""
}

// Open opens a COM port for overlapped I/O (the Go runtime's completion port
// then serves reads, writes, deadlines and Close), sets it raw 8N1 and raises
// DTR+RTS: the firmware treats DTR as "a host is listening" and says hello.
func Open(name string) (*Port, error) {
	path := name
	if !strings.HasPrefix(path, `\\.\`) {
		path = `\\.\` + name
	}
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	h, err := windows.CreateFile(p, windows.GENERIC_READ|windows.GENERIC_WRITE, 0, nil, windows.OPEN_EXISTING,
		windows.FILE_FLAG_OVERLAPPED, 0)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: name, Err: err}
	}
	if err := setupComm(h); err != nil {
		windows.CloseHandle(h)
		return nil, &os.PathError{Op: "open", Path: name, Err: err}
	}
	return &Port{Path: name, f: os.NewFile(uintptr(h), name)}, nil
}

func setupComm(h windows.Handle) error {
	var dcb windows.DCB
	dcb.DCBlength = uint32(unsafe.Sizeof(dcb))
	if err := windows.GetCommState(h, &dcb); err != nil {
		return err
	}
	// The baud rate means nothing on USB CDC, but usbser.sys wants a valid one.
	dcb.BaudRate = 115200
	dcb.ByteSize = 8
	dcb.Parity = windows.NOPARITY
	dcb.StopBits = windows.ONESTOPBIT
	const (
		fBinary          = 1 << 0
		fDtrControlMask  = 3 << 4
		fDtrControlOn    = 1 << 4
		fOutxCtsFlow     = 1 << 2
		fOutxDsrFlow     = 1 << 3
		fDsrSensitivity  = 1 << 6
		fOutX            = 1 << 8
		fInX             = 1 << 9
		fRtsControlMask  = 3 << 12
		fRtsControlOn    = 1 << 12
		fAbortOnError    = 1 << 14
		fNullAndParities = 1<<1 | 1<<11 // fParity, fNull
	)
	dcb.Flags &^= fDtrControlMask | fRtsControlMask | fOutxCtsFlow | fOutxDsrFlow | fDsrSensitivity | fOutX | fInX | fAbortOnError | fNullAndParities
	dcb.Flags |= fBinary | fDtrControlOn | fRtsControlOn
	if err := windows.SetCommState(h, &dcb); err != nil {
		return err
	}
	// A read returns as soon as at least one byte is there and otherwise waits
	// (practically) forever; writes never time out in the driver (Send sets a deadline).
	to := windows.CommTimeouts{ReadIntervalTimeout: 0xFFFFFFFF, ReadTotalTimeoutMultiplier: 0xFFFFFFFF, ReadTotalTimeoutConstant: 0xFFFFFFFE}
	if err := windows.SetCommTimeouts(h, &to); err != nil {
		return err
	}
	_ = windows.PurgeComm(h, windows.PURGE_RXCLEAR|windows.PURGE_TXCLEAR)
	_ = windows.EscapeCommFunction(h, windows.SETDTR)
	_ = windows.EscapeCommFunction(h, windows.SETRTS)
	return nil
}

// ---- hotplug ------------------------------------------------------------------

var (
	cfgmgr32                     = windows.NewLazySystemDLL("cfgmgr32.dll")
	procCMRegisterNotification   = cfgmgr32.NewProc("CM_Register_Notification")
	procCMUnregisterNotification = cfgmgr32.NewProc("CM_Unregister_Notification")

	hotplug struct {
		once sync.Once
		cb   uintptr
		mu   sync.Mutex
		ch   chan struct{}
	}
	// Device interface classes we wake up for: COM ports and USB devices.
	guidComPort   = windows.GUID{Data1: 0x86e0d1e0, Data2: 0x8089, Data3: 0x11d0, Data4: [8]byte{0x9c, 0xe4, 0x08, 0x00, 0x3e, 0x30, 0x1f, 0x73}}
	guidUSBDevice = windows.GUID{Data1: 0xa5dcbf10, Data2: 0x6530, Data3: 0x11d2, Data4: [8]byte{0x90, 0x1f, 0x00, 0xc0, 0x4f, 0xb9, 0x51, 0xed}}
)

// cmNotifyFilter is CM_NOTIFY_FILTER for CM_NOTIFY_FILTER_TYPE_DEVICEINTERFACE
// (the union is 400 bytes: WCHAR InstanceId[200]).
type cmNotifyFilter struct {
	size       uint32
	flags      uint32
	filterType uint32
	reserved   uint32
	classGUID  windows.GUID
	_          [400 - 16]byte
}

func hotplugCallback(notify, context, action, data, size uintptr) uintptr {
	hotplug.mu.Lock()
	if hotplug.ch != nil {
		select {
		case hotplug.ch <- struct{}{}:
		default:
		}
	}
	hotplug.mu.Unlock()
	return 0
}

// WaitHotplug blocks until a COM port or USB device arrives or leaves
// (CM_Register_Notification: Windows calls us, nothing polls), or until done
// is closed.
func WaitHotplug(done <-chan struct{}) {
	hotplug.once.Do(func() { hotplug.cb = syscall.NewCallback(hotplugCallback) })
	ch := make(chan struct{}, 1)
	hotplug.mu.Lock()
	hotplug.ch = ch
	hotplug.mu.Unlock()
	defer func() {
		hotplug.mu.Lock()
		hotplug.ch = nil
		hotplug.mu.Unlock()
	}()
	var handles []uintptr
	for _, g := range []windows.GUID{guidComPort, guidUSBDevice} {
		f := cmNotifyFilter{classGUID: g}
		f.size = uint32(unsafe.Sizeof(f))
		var h uintptr
		if r, _, _ := procCMRegisterNotification.Call(uintptr(unsafe.Pointer(&f)), 0, hotplug.cb, uintptr(unsafe.Pointer(&h))); r == 0 {
			handles = append(handles, h)
		}
	}
	defer func() {
		for _, h := range handles {
			_, _, _ = procCMUnregisterNotification.Call(h)
		}
	}()
	if len(handles) == 0 {
		sleep(done, 5*time.Second)
		return
	}
	select {
	case <-done:
	case <-ch:
		// The port name appears in SERIALCOMM a moment after the interface.
		sleep(done, 700*time.Millisecond)
	}
}
