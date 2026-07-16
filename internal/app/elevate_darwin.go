//go:build darwin

package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// RelaunchElevated restarts this process with administrator privileges
// (macOS authorization dialog). Caller should exit the current process on success.
//
// HOME/USER are preserved so ~/.swellbox stays the real user data dir (root's
// home would otherwise be /var/root).
func RelaunchElevated() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if r, err2 := filepath.EvalSymlinks(exe); err2 == nil {
		exe = r
	}

	home, _ := os.UserHomeDir()
	user := os.Getenv("USER")
	if user == "" {
		user = os.Getenv("LOGNAME")
	}

	// nohup so osascript returns after spawn; preserve identity for data paths.
	// quoted form of keeps paths with spaces safe in the shell.
	script := fmt.Sprintf(
		`do shell script "export HOME=" & quoted form of %s & " USER=" & quoted form of %s & " LOGNAME=" & quoted form of %s & "; /usr/bin/nohup " & quoted form of %s & " >/dev/null 2>&1 &" with administrator privileges`,
		appleString(home),
		appleString(user),
		appleString(user),
		appleString(exe),
	)
	out, err := exec.Command("osascript", "-e", script).CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out)) + " " + err.Error()
		low := strings.ToLower(msg)
		if strings.Contains(low, "user canceled") ||
			strings.Contains(low, "user cancelled") ||
			strings.Contains(msg, "(-128)") {
			return fmt.Errorf("uac cancelled")
		}
		return fmt.Errorf("macOS elevation failed: %s", strings.TrimSpace(string(out)+" "+err.Error()))
	}
	return nil
}
