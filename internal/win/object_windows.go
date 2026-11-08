package win

import (
	"sync"
	"sync/atomic"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// COM objects implemented in Go: event sinks and callbacks that Windows calls
// into (audio volume changes, media session events…).
//
// An Object's first word is its vtable, like any COM object. Objects are kept
// in a package list for the life of the process: they are few, created once,
// and Windows may call them until it drops its last reference.

// Method is one vtable slot after IUnknown's three. Args are the parameters
// after `this`; float parameters arrive in XMM registers and read as garbage.
type Method struct {
	Args int
	Fn   func(args []uintptr) uintptr
}

// Object is a COM object whose methods run Go code.
type Object struct {
	vtbl    *uintptr
	refs    int32
	iids    []windows.GUID
	methods []Method
}

var (
	iidIUnknown    = IID("00000000-0000-0000-c000-000000000046")
	iidIAgile      = IID("94ea2b94-e9cc-49e0-c0ff-ee64ca8f5b90")
	objects        sync.Mutex
	alive          []*Object
	vtables        = map[string][]uintptr{} // by arity pattern
	unknownSlots   [3]uintptr
	unknownOnce    sync.Once
	dispatchByArgs = map[[2]int]uintptr{} // (slot, args) -> callback
)

func objectOf(this uintptr) *Object { return (*Object)(Ptr(this)) }

func initUnknown() {
	unknownSlots[0] = syscall.NewCallback(func(this, riid, ppv uintptr) uintptr {
		if ppv == 0 {
			return E_POINTER
		}
		o := objectOf(this)
		want := *(*windows.GUID)(Ptr(riid))
		ok := want == *iidIUnknown || want == *iidIAgile
		for _, g := range o.iids {
			ok = ok || want == g
		}
		if !ok {
			*(*uintptr)(Ptr(ppv)) = 0
			return E_NOINTERFACE
		}
		atomic.AddInt32(&o.refs, 1)
		*(*uintptr)(Ptr(ppv)) = this
		return 0
	})
	unknownSlots[1] = syscall.NewCallback(func(this uintptr) uintptr {
		return uintptr(atomic.AddInt32(&objectOf(this).refs, 1))
	})
	unknownSlots[2] = syscall.NewCallback(func(this uintptr) uintptr {
		return uintptr(max(1, atomic.AddInt32(&objectOf(this).refs, -1))) // never freed
	})
}

// slot returns the shared callback for vtable slot i (after IUnknown) taking n args.
func slot(i, n int) uintptr {
	key := [2]int{i, n}
	if cb, ok := dispatchByArgs[key]; ok {
		return cb
	}
	call := func(this uintptr, a ...uintptr) uintptr {
		o := objectOf(this)
		if i >= len(o.methods) {
			return 0
		}
		return o.methods[i].Fn(a)
	}
	var cb uintptr
	switch n {
	case 0:
		cb = syscall.NewCallback(func(this uintptr) uintptr { return call(this) })
	case 1:
		cb = syscall.NewCallback(func(this, a uintptr) uintptr { return call(this, a) })
	case 2:
		cb = syscall.NewCallback(func(this, a, b uintptr) uintptr { return call(this, a, b) })
	case 3:
		cb = syscall.NewCallback(func(this, a, b, c uintptr) uintptr { return call(this, a, b, c) })
	case 4:
		cb = syscall.NewCallback(func(this, a, b, c, d uintptr) uintptr { return call(this, a, b, c, d) })
	default:
		cb = syscall.NewCallback(func(this, a, b, c, d, e uintptr) uintptr { return call(this, a, b, c, d, e) })
	}
	dispatchByArgs[key] = cb
	return cb
}

// NewObject builds a COM object answering QueryInterface for IUnknown,
// IAgileObject and iids (its methods may run on any thread).
func NewObject(iids []*windows.GUID, methods ...Method) Obj {
	objects.Lock()
	defer objects.Unlock()
	unknownOnce.Do(initUnknown)
	key := ""
	for _, m := range methods {
		key += string(rune('0' + m.Args))
	}
	vt, ok := vtables[key]
	if !ok {
		vt = append([]uintptr{}, unknownSlots[:]...)
		for i, m := range methods {
			vt = append(vt, slot(i, m.Args))
		}
		vtables[key] = vt
	}
	o := &Object{vtbl: &vt[0], refs: 1, methods: methods}
	for _, g := range iids {
		o.iids = append(o.iids, *g)
	}
	alive = append(alive, o)
	return Obj(unsafe.Pointer(o))
}

// Delegate is a WinRT delegate (TypedEventHandler and the like): one Invoke
// method taking sender and args.
func Delegate(iid string, fn func(sender, args Obj)) Obj {
	return NewObject([]*windows.GUID{IID(iid)}, Method{Args: 2, Fn: func(a []uintptr) uintptr {
		fn(Obj(a[0]), Obj(a[1]))
		return 0
	}})
}
