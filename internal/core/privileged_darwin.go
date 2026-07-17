//go:build darwin

package core

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// startPrivileged launches sing-box as root via macOS authorization (password dialog).
// Used for TUN when the tray app itself is not running as root.
func (m *Manager) startPrivileged(bin, configPath, workDir, logPath string) error {
	// Append to the same core.log; create/touch first so non-root can read it.
	// Caller holds m.mu; unlock on every return path.
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		m.mu.Unlock()
		return err
	}
	_ = f.Close()

	// One osascript call that:
	//   1. kills any existing sing-box (pkill -9 -x sing-box) so we never
	//      get "port already in use" or "cache-file: timeout" from a stale
	//      root process — no extra password prompt because it's all one script.
	//   2. Starts a fresh sing-box in the background, echoes its PID.
	// Avoid nohup: it requires a controlling TTY and crashes when launched
	// from a .app bundle. sh -c '... &' works without a TTY.
	// quoted form of keeps paths with spaces/special chars safe in AppleScript.
	script := fmt.Sprintf(
		`do shell script "pkill -9 -x sing-box 2>/dev/null; sleep 0.3; sh -c " & quoted form of (quoted form of %s & " run -c " & quoted form of %s & " -D " & quoted form of %s & " >> " & quoted form of %s & " 2>&1 & echo $!") with administrator privileges`,
		asQuote(bin),
		asQuote(configPath),
		asQuote(workDir),
		asQuote(logPath),
	)
	out, err := exec.Command("osascript", "-e", script).CombinedOutput()
	if err != nil {
		m.mu.Unlock()
		msg := strings.TrimSpace(string(out))
		low := strings.ToLower(msg + " " + err.Error())
		if strings.Contains(low, "user canceled") ||
			strings.Contains(low, "user cancelled") ||
			strings.Contains(msg, "(-128)") {
			return fmt.Errorf("authorization cancelled")
		}
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("privileged start failed: %s", msg)
	}
	pidStr := strings.TrimSpace(string(out))
	// osascript may wrap output; take last integer token
	pid := 0
	for _, part := range strings.Fields(pidStr) {
		if n, e := strconv.Atoi(part); e == nil && n > 0 {
			pid = n
		}
	}
	if pid <= 0 {
		m.mu.Unlock()
		return fmt.Errorf("privileged start: no pid in output %q", pidStr)
	}

	m.cmd = nil
	m.logFile = nil
	m.privilegedPID = pid
	m.privileged = true
	m.running = true
	m.mu.Unlock()

	// Reap when process disappears (not our child — cannot Wait).
	go func(p int) {
		for {
			time.Sleep(500 * time.Millisecond)
			if !pidAlive(p) {
				m.mu.Lock()
				if m.privilegedPID == p {
					m.running = false
					m.privileged = false
					m.privilegedPID = 0
				}
				m.mu.Unlock()
				return
			}
		}
	}(pid)

	time.Sleep(400 * time.Millisecond)
	m.mu.Lock()
	running := m.running && pidAlive(pid)
	m.mu.Unlock()
	if !running {
		return fmt.Errorf("sing-box exited immediately after privileged start; see %s", logPath)
	}
	return nil
}


func (m *Manager) stopPrivileged() error {
	m.mu.Lock()
	pid := m.privilegedPID
	m.running = false
	m.privileged = false
	m.privilegedPID = 0
	m.cmd = nil
	m.mu.Unlock()
	return killPrivilegedPID(pid)
}

// killPrivilegedPID kills a root-owned sing-box process.
// Strategy: try SIGTERM first (no-op if cross-user), then use osascript
// to run pkill/kill -9 as admin. pkill by executable name is a reliable
// fallback that doesn't require knowing the exact PID.
func killPrivilegedPID(pid int) error {
	if pid > 0 {
		// Try cheap SIGTERM (works if we're the same user — usually fails for root).
		if proc, err := os.FindProcess(pid); err == nil {
			_ = proc.Signal(syscall.SIGTERM)
			time.Sleep(400 * time.Millisecond)
			if !pidAlive(pid) {
				return nil
			}
		}
	}
	// Use osascript to run kill as admin — no interactive password needed
	// because macOS caches the authorization credential from the start dialog.
	// NOTE: do NOT quote the process name inside the AppleScript string literal;
	// pkill accepts an unquoted name and %q would add extra quotes that break
	// the AppleScript string delimiters.
	coreName := "sing-box"
	var script string
	if pid > 0 {
		script = fmt.Sprintf(
			`do shell script "kill -15 %d 2>/dev/null; sleep 0.5; kill -9 %d 2>/dev/null; pkill -9 -x %s 2>/dev/null; true" with administrator privileges`,
			pid, pid, coreName,
		)
	} else {
		script = fmt.Sprintf(
			`do shell script "pkill -15 -x %s 2>/dev/null; sleep 0.5; pkill -9 -x %s 2>/dev/null; true" with administrator privileges`,
			coreName, coreName,
		)
	}
	out, err := exec.Command("osascript", "-e", script).CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		low := strings.ToLower(msg + " " + err.Error())
		if strings.Contains(low, "user canceled") || strings.Contains(low, "user cancelled") || strings.Contains(msg, "(-128)") {
			// User cancelled — best effort; process may linger.
			return fmt.Errorf("authorization cancelled while stopping privileged core")
		}
		if pid <= 0 || !pidAlive(pid) {
			return nil
		}
		return fmt.Errorf("stop privileged core: %s", msg)
	}
	return nil
}

// KillAllPrivileged kills any running root sing-box processes before a fresh start.
// Call this from startProxy on macOS TUN to ensure stale root processes don't
// block port binding or SQLite cache file access.
func KillAllPrivileged() {
	_ = killPrivilegedPID(0)
	time.Sleep(800 * time.Millisecond)
}



func pidAlive(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = proc.Signal(syscall.Signal(0))
	if err == nil {
		return true
	}
	// On macOS, sending signal 0 to a root-owned process from a normal user
	// returns EPERM (operation not permitted). This means the process is alive.
	if errors.Is(err, syscall.EPERM) {
		return true
	}
	// "os: process already finished" or ESRCH
	return false
}

func asQuote(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return `"` + s + `"`
}
