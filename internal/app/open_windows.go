//go:build windows

package app

import (
	"fmt"
	"os/exec"
	"syscall"
	"unsafe"
)

func openURL(url string) error {
	// 1) ShellExecuteW — most reliable on interactive desktop
	if err := shellExecute(url); err == nil {
		return nil
	}

	// 2) fallback: powershell Start-Process
	cmd := exec.Command("powershell", "-NoProfile", "-WindowStyle", "Hidden",
		"-Command", "Start-Process", url)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := cmd.Start(); err == nil {
		return nil
	}

	// 3) fallback: cmd start
	cmd = exec.Command("cmd", "/c", "start", "", url)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("open url: %w", err)
	}
	return nil
}

func openPath(path string) error {
	if err := shellExecute(path); err == nil {
		return nil
	}
	cmd := exec.Command("explorer", path)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("open path: %w", err)
	}
	return nil
}

func openInNotepad(path string) error {
	// Prefer classic Notepad so JSON is editable as plain text.
	cmd := exec.Command("notepad.exe", path)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: false}
	if err := cmd.Start(); err != nil {
		// Fallback: default associated app
		return openPath(path)
	}
	return nil
}

func shellExecute(file string) error {
	verb, err := syscall.UTF16PtrFromString("open")
	if err != nil {
		return err
	}
	filePtr, err := syscall.UTF16PtrFromString(file)
	if err != nil {
		return err
	}
	shell32 := syscall.NewLazyDLL("shell32.dll")
	proc := shell32.NewProc("ShellExecuteW")
	// HINSTANCE > 32 means success
	r, _, _ := proc.Call(
		0,
		uintptr(unsafe.Pointer(verb)),
		uintptr(unsafe.Pointer(filePtr)),
		0,
		0,
		1, // SW_SHOWNORMAL
	)
	if r <= 32 {
		return fmt.Errorf("ShellExecuteW failed: code %d", r)
	}
	return nil
}
