//go:build windows

package core

import (
	"os/exec"
	"syscall"
)

func hideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}
}

func terminate(cmd *exec.Cmd) error {
	// On Windows, Process.Kill is the practical approach for console-less children.
	return cmd.Process.Kill()
}
