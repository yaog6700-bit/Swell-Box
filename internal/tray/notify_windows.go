//go:build windows

package tray

import (
	"syscall"
	"unsafe"
)

func popup(title, message string) {
	t, _ := syscall.UTF16PtrFromString(title)
	m, _ := syscall.UTF16PtrFromString(message)
	user32 := syscall.NewLazyDLL("user32.dll")
	proc := user32.NewProc("MessageBoxW")
	// MB_OK | MB_ICONINFORMATION | MB_TOPMOST
	proc.Call(0, uintptr(unsafe.Pointer(m)), uintptr(unsafe.Pointer(t)), 0x00000040|0x00040000)
}
