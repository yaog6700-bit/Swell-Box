//go:build windows

package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

// RelaunchElevated restarts the current executable with a UAC elevation prompt (runas).
// On success the caller should exit the current (non-elevated) process.
func RelaunchElevated() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if r, err2 := filepath.EvalSymlinks(exe); err2 == nil {
		exe = r
	}
	verb, err := syscall.UTF16PtrFromString("runas")
	if err != nil {
		return err
	}
	filePtr, err := syscall.UTF16PtrFromString(exe)
	if err != nil {
		return err
	}
	// Preserve original args (skip argv0).
	var params *uint16
	if len(os.Args) > 1 {
		p := strings.Join(quoteArgs(os.Args[1:]), " ")
		params, err = syscall.UTF16PtrFromString(p)
		if err != nil {
			return err
		}
	}
	dirPtr, err := syscall.UTF16PtrFromString(filepath.Dir(exe))
	if err != nil {
		return err
	}
	shell32 := syscall.NewLazyDLL("shell32.dll")
	proc := shell32.NewProc("ShellExecuteW")
	// SW_SHOWNORMAL = 1; return value > 32 means success
	r, _, _ := proc.Call(
		0,
		uintptr(unsafe.Pointer(verb)),
		uintptr(unsafe.Pointer(filePtr)),
		uintptr(unsafe.Pointer(params)),
		uintptr(unsafe.Pointer(dirPtr)),
		1,
	)
	if r <= 32 {
		// 1223 = ERROR_CANCELLED (user denied UAC)
		if r == 1223 {
			return fmt.Errorf("uac cancelled")
		}
		return fmt.Errorf("ShellExecuteW runas failed: code %d", r)
	}
	return nil
}

func quoteArgs(args []string) []string {
	out := make([]string, 0, len(args))
	for _, a := range args {
		if a == "" {
			out = append(out, `""`)
			continue
		}
		if strings.ContainsAny(a, " \t\"") {
			a = `"` + strings.ReplaceAll(a, `"`, `\"`) + `"`
		}
		out = append(out, a)
	}
	return out
}
