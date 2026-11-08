package win

import (
	"errors"
	"fmt"
	"runtime"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	ole32   = windows.NewLazySystemDLL("ole32.dll")
	combase = windows.NewLazySystemDLL("combase.dll")

	procCoInitializeEx   = ole32.NewProc("CoInitializeEx")
	procCoCreateInstance = ole32.NewProc("CoCreateInstance")
	procCoTaskMemFree    = ole32.NewProc("CoTaskMemFree")
	procPropVariantClear = ole32.NewProc("PropVariantClear")
	procRoInitialize     = combase.NewProc("RoInitialize")
	procRoGetActivation  = combase.NewProc("RoGetActivationFactory")
	procWindowsCreateStr = combase.NewProc("WindowsCreateString")
	procWindowsDeleteStr = combase.NewProc("WindowsDeleteString")
	procWindowsGetRawBuf = combase.NewProc("WindowsGetStringRawBuffer")
)

// Ptr turns an address handed out by Windows into a pointer. The double
// indirection keeps vet's unsafeptr check quiet: the memory is not Go's.
func Ptr(u uintptr) unsafe.Pointer { return *(*unsafe.Pointer)(unsafe.Pointer(&u)) }

// HRESULT is a failed COM call.
type HRESULT uint32

func (h HRESULT) Error() string { return fmt.Sprintf("HRESULT 0x%08X", uint32(h)) }

// HR turns a returned HRESULT into an error (nil for S_OK, S_FALSE…).
func HR(r uintptr) error {
	if int32(uint32(r)) < 0 {
		return HRESULT(uint32(r))
	}
	return nil
}

const (
	E_NOINTERFACE = 0x80004002
	E_POINTER     = 0x80004003
)

// IID parses "xxxxxxxx-xxxx-…" (no braces) and panics on a typo.
func IID(s string) *windows.GUID {
	g, err := windows.GUIDFromString("{" + s + "}")
	if err != nil {
		panic(err)
	}
	return &g
}

// Obj is a COM interface pointer: its first word points at the vtable.
type Obj uintptr

// Call invokes vtable slot i (0..2 are IUnknown's, 3..5 IInspectable's) and
// returns the raw HRESULT (or whatever the method returns). Like
// LazyProc.Call, pointers converted to uintptr in its arguments stay alive
// and in place for the call (go:uintptrescapes).
//
//go:uintptrescapes
func (o Obj) Call(i int, args ...uintptr) uintptr {
	vt := *(*uintptr)(Ptr(uintptr(o)))
	fn := *(*uintptr)(Ptr(vt + uintptr(i)*unsafe.Sizeof(uintptr(0))))
	r, _, _ := syscall.SyscallN(fn, append([]uintptr{uintptr(o)}, args...)...)
	return r
}

// CallErr is Call with the result as an error.
//
//go:uintptrescapes
func (o Obj) CallErr(i int, args ...uintptr) error { return HR(o.Call(i, args...)) }

// Get calls a method whose only argument is an out pointer and returns what it wrote.
func (o Obj) Get(i int) (Obj, error) {
	var out Obj
	if err := HR(o.Call(i, uintptr(unsafe.Pointer(&out)))); err != nil {
		return 0, err
	}
	if out == 0 {
		return 0, HRESULT(E_POINTER)
	}
	return out, nil
}

func (o Obj) AddRef() {
	if o != 0 {
		o.Call(1)
	}
}

func (o Obj) Release() {
	if o != 0 {
		o.Call(2)
	}
}

func (o Obj) QueryInterface(iid *windows.GUID) (Obj, error) {
	var out Obj
	if err := HR(o.Call(0, uintptr(unsafe.Pointer(iid)), uintptr(unsafe.Pointer(&out)))); err != nil {
		return 0, err
	}
	return out, nil
}

var mta struct {
	once sync.Once
	err  error
}

// InitMTA makes the process's multithreaded apartment exist for good: one
// locked OS thread joins it and parks. Every other thread is then implicitly
// in the MTA, so goroutines can call free-threaded COM and WinRT objects
// from whichever thread they run on.
func InitMTA() error {
	mta.once.Do(func() {
		ready := make(chan error)
		go func() {
			runtime.LockOSThread() // never unlocked: the thread stays in the MTA
			const COINIT_MULTITHREADED = 0
			r, _, _ := procCoInitializeEx.Call(0, COINIT_MULTITHREADED)
			if err := HR(r); err != nil {
				ready <- err
				return
			}
			_, _, _ = procRoInitialize.Call(1) // RO_INIT_MULTITHREADED; S_FALSE once COM is up
			ready <- nil
			select {}
		}()
		mta.err = <-ready
	})
	return mta.err
}

// InitSTA puts the calling (locked) thread in a single-threaded apartment,
// for shell calls such as ShellExecute.
func InitSTA() {
	const COINIT_APARTMENTTHREADED, COINIT_DISABLE_OLE1DDE = 2, 4
	_, _, _ = procCoInitializeEx.Call(0, COINIT_APARTMENTTHREADED|COINIT_DISABLE_OLE1DDE)
}

// CreateInstance is CoCreateInstance for an in-process server.
func CreateInstance(clsid, iid *windows.GUID) (Obj, error) {
	if err := InitMTA(); err != nil {
		return 0, err
	}
	const CLSCTX_ALL = 0x17
	var out Obj
	r, _, _ := procCoCreateInstance.Call(uintptr(unsafe.Pointer(clsid)), 0, CLSCTX_ALL, uintptr(unsafe.Pointer(iid)), uintptr(unsafe.Pointer(&out)))
	if err := HR(r); err != nil {
		return 0, err
	}
	return out, nil
}

// TaskMemFree frees memory COM allocated for us (strings it returned).
func TaskMemFree(p uintptr) {
	if p != 0 {
		_, _, _ = procCoTaskMemFree.Call(p)
	}
}

// PropVariantClear frees a PROPVARIANT's contents.
func PropVariantClear(pv unsafe.Pointer) { _, _, _ = procPropVariantClear.Call(uintptr(pv)) }

// UTF16At reads a NUL-terminated UTF-16 string at an address Windows gave us.
func UTF16At(p uintptr) string {
	if p == 0 {
		return ""
	}
	return windows.UTF16PtrToString((*uint16)(Ptr(p)))
}

// ---- WinRT ------------------------------------------------------------------

// HString is a WinRT string handle.
type HString uintptr

func NewHString(s string) (HString, error) {
	u, err := windows.UTF16FromString(s)
	if err != nil {
		return 0, err
	}
	var h HString
	r, _, _ := procWindowsCreateStr.Call(uintptr(unsafe.Pointer(&u[0])), uintptr(len(u)-1), uintptr(unsafe.Pointer(&h)))
	if err := HR(r); err != nil {
		return 0, err
	}
	return h, nil
}

func (h HString) Delete() {
	if h != 0 {
		_, _, _ = procWindowsDeleteStr.Call(uintptr(h))
	}
}

// String reads the string; a null handle is "".
func (h HString) String() string {
	if h == 0 {
		return ""
	}
	var n uint32
	p, _, _ := procWindowsGetRawBuf.Call(uintptr(h), uintptr(unsafe.Pointer(&n)))
	if p == 0 || n == 0 {
		return ""
	}
	return windows.UTF16ToString(unsafe.Slice((*uint16)(Ptr(p)), n))
}

// GetString calls a WinRT property getter returning an HSTRING and frees it.
func (o Obj) GetString(i int) (string, error) {
	var h HString
	if err := HR(o.Call(i, uintptr(unsafe.Pointer(&h)))); err != nil {
		return "", err
	}
	defer h.Delete()
	return h.String(), nil
}

// ActivationFactory returns a runtime class's factory (or statics) interface.
func ActivationFactory(class string, iid *windows.GUID) (Obj, error) {
	if err := InitMTA(); err != nil {
		return 0, err
	}
	h, err := NewHString(class)
	if err != nil {
		return 0, err
	}
	defer h.Delete()
	var out Obj
	r, _, _ := procRoGetActivation.Call(uintptr(h), uintptr(unsafe.Pointer(iid)), uintptr(unsafe.Pointer(&out)))
	if err := HR(r); err != nil {
		return 0, err
	}
	return out, nil
}

var iidAsyncInfo = IID("00000036-0000-0000-c000-000000000046")

// Await waits for a WinRT IAsyncOperation/IAsyncAction to finish and, for an
// operation, returns its result (GetResults is slot 8 on both; an action's
// GetResults takes no out pointer, pass wantResult=false). The operation is
// released. Async calls here are one-offs started by an event or a user
// action, so a short status poll (no completion delegate to implement)
// costs nothing while idle.
func Await(op Obj, timeout time.Duration, wantResult bool) (Obj, error) {
	defer op.Release()
	info, err := op.QueryInterface(iidAsyncInfo)
	if err != nil {
		return 0, err
	}
	defer info.Release()
	deadline := time.Now().Add(timeout)
	wait := time.Millisecond
	for {
		var status int32 // Started 0, Completed 1, Canceled 2, Error 3
		if err := HR(info.Call(7, uintptr(unsafe.Pointer(&status)))); err != nil {
			return 0, err
		}
		switch status {
		case 1:
			if !wantResult {
				return 0, HR(op.Call(8))
			}
			return op.Get(8)
		case 2:
			return 0, errors.New("async operation canceled")
		case 3:
			var code uint32
			_ = info.CallErr(8, uintptr(unsafe.Pointer(&code)))
			return 0, HRESULT(code)
		}
		if time.Now().After(deadline) {
			_ = info.CallErr(9) // Cancel
			return 0, errors.New("async operation timed out")
		}
		time.Sleep(wait)
		wait = min(wait*2, 50*time.Millisecond)
	}
}
