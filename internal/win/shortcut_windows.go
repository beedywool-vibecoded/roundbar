package win

import (
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	clsidShellLink  = IID("00021401-0000-0000-c000-000000000046")
	iidIShellLinkW  = IID("000214f9-0000-0000-c000-000000000046")
	iidIPersistFile = IID("0000010b-0000-0000-c000-000000000046")
)

// CreateShortcut writes a .lnk file (IShellLinkW + IPersistFile), on an STA
// thread of its own.
func CreateShortcut(lnk, target, args, description string) error {
	done := make(chan error, 1)
	go func() {
		runtime.LockOSThread() // never unlocked: the thread ends with this goroutine
		InitSTA()
		done <- createShortcut(lnk, target, args, description)
	}()
	return <-done
}

func createShortcut(lnk, target, args, description string) error {
	const CLSCTX_INPROC_SERVER = 1
	var sl Obj
	r, _, _ := procCoCreateInstance.Call(uintptr(unsafe.Pointer(clsidShellLink)), 0, CLSCTX_INPROC_SERVER,
		uintptr(unsafe.Pointer(iidIShellLinkW)), uintptr(unsafe.Pointer(&sl)))
	if err := HR(r); err != nil {
		return err
	}
	defer sl.Release()
	str := func(s string) *uint16 {
		p, _ := windows.UTF16PtrFromString(s)
		return p
	}
	t, a, d, l := str(target), str(args), str(description), str(lnk)
	if err := sl.CallErr(20, uintptr(unsafe.Pointer(t))); err != nil { // SetPath
		return err
	}
	_ = sl.CallErr(11, uintptr(unsafe.Pointer(a)))    // SetArguments
	_ = sl.CallErr(7, uintptr(unsafe.Pointer(d)))     // SetDescription
	_ = sl.CallErr(17, uintptr(unsafe.Pointer(t)), 0) // SetIconLocation
	pf, err := sl.QueryInterface(iidIPersistFile)
	if err != nil {
		return err
	}
	defer pf.Release()
	return pf.CallErr(6, uintptr(unsafe.Pointer(l)), 1) // Save(path, remember)
}
