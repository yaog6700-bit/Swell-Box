//go:build windows

package app

import (
	"golang.org/x/sys/windows"
)

// singleInstanceMutex keeps exactly one Swell-Box process per user session.
var singleInstanceMutex windows.Handle

// AcquireSingleInstance returns false if another Swell-Box is already running.
func AcquireSingleInstance() (ok bool, err error) {
	name, err := windows.UTF16PtrFromString(`Local\Swell-Box-single-instance`)
	if err != nil {
		return false, err
	}
	// bInitialOwner=false: we only care about uniqueness, not ownership semantics.
	h, err := windows.CreateMutex(nil, false, name)
	if h == 0 {
		return false, err
	}
	// Existing mutex → another instance is alive.
	if err == windows.ERROR_ALREADY_EXISTS {
		_ = windows.CloseHandle(h)
		return false, nil
	}
	singleInstanceMutex = h
	return true, nil
}

// ReleaseSingleInstance unlocks so a future launch can start.
func ReleaseSingleInstance() {
	if singleInstanceMutex != 0 {
		_ = windows.CloseHandle(singleInstanceMutex)
		singleInstanceMutex = 0
	}
}
