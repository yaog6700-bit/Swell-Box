//go:build darwin

package core

import (
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

	// Shell: nohup … & echo $!  — returns PID to osascript stdout.
	// quoted form of keeps spaces/special chars safe.
	script := fmt.Sprintf(
		`do shell script " /usr/bin/nohup " & quoted form of %s & " run -c " & quoted form of %s & " -D " & quoted form of %s & " >> " & quoted form of %s & " 2>&1 & echo $!" with administrator privileges`,
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
	if pid <= 0 {
		return nil
	}
	// Try unprivileged signal first (works if same user — usually not for root).
	if proc, err := os.FindProcess(pid); err == nil {
		_ = proc.Signal(syscall.SIGTERM)
		time.Sleep(300 * time.Millisecond)
		if !pidAlive(pid) {
			return nil
		}
	}
	// Root-owned process: need authorization to kill.
	script := fmt.Sprintf(
		`do shell script "kill " & %d & " 2>/dev/null; sleep 0.3; kill -9 " & %d & " 2>/dev/null; true" with administrator privileges`,
		pid, pid,
	)
	out, err := exec.Command("osascript", "-e", script).CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		low := strings.ToLower(msg + " " + err.Error())
		if strings.Contains(low, "user canceled") || strings.Contains(low, "user cancelled") || strings.Contains(msg, "(-128)") {
			// Best-effort: process may remain until user kills it.
			return fmt.Errorf("authorization cancelled while stopping privileged core (pid %d)", pid)
		}
		// If already dead, treat as success.
		if !pidAlive(pid) {
			return nil
		}
		return fmt.Errorf("stop privileged core: %s", msg)
	}
	return nil
}

func pidAlive(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// On Unix, Signal(0) probes existence.
	err = proc.Signal(syscall.Signal(0))
	return err == nil
}

func asQuote(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return `"` + s + `"`
}
