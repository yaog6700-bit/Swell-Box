//go:build !windows

package app

import (
	"fmt"
	"os/exec"
	"runtime"
)

func openURL(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("open url: %w", err)
	}
	return nil
}

func openPath(path string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("open path: %w", err)
	}
	return nil
}

func openInNotepad(path string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", "-t", path).Start()
	default:
		// try common editors then fallback
		for _, ed := range []string{"xdg-open", "nano", "vi"} {
			if err := exec.Command(ed, path).Start(); err == nil {
				return nil
			}
		}
		return openPath(path)
	}
}
