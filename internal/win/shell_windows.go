package win

import (
	"runtime"

	"golang.org/x/sys/windows"
)

// ShellOpen hands a URL, file, folder or program to the shell (ShellExecute
// "open"), on a thread of its own in a single-threaded apartment as the shell
// wants. It waits for ShellExecute to return.
func ShellOpen(target, args string) error {
	done := make(chan error, 1)
	go func() {
		runtime.LockOSThread() // never unlocked: the thread ends with this goroutine, apartment and all
		InitSTA()
		verb, _ := windows.UTF16PtrFromString("open")
		file, err := windows.UTF16PtrFromString(target)
		if err != nil {
			done <- err
			return
		}
		var params *uint16
		if args != "" {
			params, _ = windows.UTF16PtrFromString(args)
		}
		const SW_SHOWNORMAL = 1
		done <- windows.ShellExecute(0, verb, file, params, nil, SW_SHOWNORMAL)
	}()
	return <-done
}
