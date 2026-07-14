//go:build darwin

package app

import (
	"fmt"
	"os/exec"
	"strings"
)

// PickJSONFile opens a macOS file chooser (osascript) for JSON configs.
func PickJSONFile(title string) (string, error) {
	if title == "" {
		title = "Select config"
	}
	// AppleScript — choose file
	script := fmt.Sprintf(`set theFile to choose file with prompt %q of type {"public.json", "json"}
POSIX path of theFile`, title)
	out, err := exec.Command("osascript", "-e", script).Output()
	if err != nil {
		// user cancel often returns exit 1
		if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 1 {
			return "", nil
		}
		// fallback without type filter
		script = fmt.Sprintf(`set theFile to choose file with prompt %q
POSIX path of theFile`, title)
		out, err = exec.Command("osascript", "-e", script).Output()
		if err != nil {
			if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 1 {
				return "", nil
			}
			return "", fmt.Errorf("file dialog: %w", err)
		}
	}
	return strings.TrimSpace(string(out)), nil
}
