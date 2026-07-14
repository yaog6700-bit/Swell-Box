//go:build !windows

package sharelink

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

func ReadClipboard() (string, error) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("pbpaste")
	default:
		cmd = exec.Command("xclip", "-selection", "clipboard", "-o")
	}
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("clipboard: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}
