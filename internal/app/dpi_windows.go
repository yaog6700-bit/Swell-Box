//go:build windows

package app

import (
	"golang.org/x/sys/windows"
)

// EnableDPIAwareness opts into Per-Monitor V2 so native menus/text stay sharp
// on scaled displays (125%/150%/200%). Without this, Windows bitmap-scales the
// process UI and tray popup text looks blurry.
func EnableDPIAwareness() {
	// DPI_AWARENESS_CONTEXT handles are defined as -1,-2,-3,-4 by Win32.
	// Per-monitor V2 = -4
	user32 := windows.NewLazySystemDLL("user32.dll")
	if setCtx := user32.NewProc("SetProcessDpiAwarenessContext"); setCtx.Find() == nil {
		// HANDLE(-4)
		ctx := uintptr(uncheckedUintptrFromInt(-4))
		r, _, _ := setCtx.Call(ctx)
		if r != 0 {
			return
		}
	}
	// Windows 8.1: PROCESS_PER_MONITOR_DPI_AWARE = 2
	shcore := windows.NewLazySystemDLL("shcore.dll")
	if setAwareness := shcore.NewProc("SetProcessDpiAwareness"); setAwareness.Find() == nil {
		_, _, _ = setAwareness.Call(2)
		return
	}
	// Vista+
	if setAware := user32.NewProc("SetProcessDPIAware"); setAware.Find() == nil {
		_, _, _ = setAware.Call()
	}
}

func uncheckedUintptrFromInt(v int) uintptr {
	// convert negative HANDLE values correctly on 64-bit
	return uintptr(int64(v))
}
