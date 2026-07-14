//go:build windows

package sharelink

import (
	"os/exec"
	"strings"
	"syscall"
)

// ReadClipboard returns text from the Windows clipboard.
func ReadClipboard() (string, error) {
	cmd := exec.Command("powershell", "-NoProfile", "-WindowStyle", "Hidden",
		"-Command", "Get-Clipboard -Raw")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
