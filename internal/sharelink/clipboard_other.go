//go:build !windows

package sharelink

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

func ReadClipboard() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		out, err := exec.Command("pbpaste").Output()
		if err != nil {
			return "", fmt.Errorf("clipboard: %w", err)
		}
		return strings.TrimSpace(string(out)), nil
	default:
		// Linux: try Wayland then X11 tools
		for _, attempt := range [][]string{
			{"wl-paste", "--no-newline"},
			{"xclip", "-selection", "clipboard", "-o"},
			{"xsel", "--clipboard", "--output"},
		} {
			if _, err := exec.LookPath(attempt[0]); err != nil {
				continue
			}
			out, err := exec.Command(attempt[0], attempt[1:]...).Output()
			if err != nil {
				continue
			}
			return strings.TrimSpace(string(out)), nil
		}
		return "", fmt.Errorf("clipboard: need wl-paste, xclip, or xsel")
	}
}
