//go:build !windows

package core

import (
	"os/exec"
	"syscall"
)

func hideWindow(cmd *exec.Cmd) {
	// no-op on unix
}

func terminate(cmd *exec.Cmd) error {
	// Prefer graceful SIGTERM.
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		return cmd.Process.Kill()
	}
	return nil
}
