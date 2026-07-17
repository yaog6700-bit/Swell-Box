package core

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/swell-app/swellbox/internal/paths"
)

// Manager runs the official sing-box binary as a child process.
type Manager struct {
	mu      sync.Mutex
	cmd     *exec.Cmd
	running bool
	logFile *os.File

	// CorePath overrides auto detection when non-empty.
	CorePath string
	// WorkDir is passed as sing-box -D (config directory context).
	WorkDir string
	// ConfigPath is the runtime config file passed as -c.
	ConfigPath string
	// NeedPrivileges requests elevated start (macOS TUN: admin password dialog).
	// When true and the process is not already root/admin, Start may use a
	// platform-specific privileged launcher (see privileged_*.go).
	NeedPrivileges bool

	// privileged marks a core started via admin helper (not a normal child).
	privileged    bool
	privilegedPID int
}

func (m *Manager) Running() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.running
}

// ResolveBinary finds the sing-box executable that will actually run.
//
// Single source of truth: ~/.swellbox/bin (Windows: %USERPROFILE%\.swellbox\bin).
// The copy next to Swell-Box.exe in a full.zip is only a first-run seed —
// InstallBundledCore / EnsureCore copy it into the data dir; it is never used
// as a second runtime path (that caused "update said latest, Dashboard old").
//
// Order: explicit CorePath (if set and valid) → data-dir bin → PATH (dev only).
func (m *Manager) ResolveBinary() (string, error) {
	name := paths.CoreBinaryName()

	if m.CorePath != "" {
		if st, err := os.Stat(m.CorePath); err == nil && !st.IsDir() && st.Size() > 0 {
			return m.CorePath, nil
		}
		return "", fmt.Errorf("core_path not found: %s", m.CorePath)
	}

	// Canonical install location for tray updates and first-run seed.
	if binDir, err := paths.BinDir(); err == nil {
		candidate := filepath.Join(binDir, name)
		if st, err := os.Stat(candidate); err == nil && !st.IsDir() && st.Size() > 0 {
			return candidate, nil
		}
	}

	// Dev / manual: PATH only (not the zip-side binary).
	if p, err := exec.LookPath(name); err == nil {
		return p, nil
	}
	if runtime.GOOS == "windows" {
		if p, err := exec.LookPath("sing-box"); err == nil {
			return p, nil
		}
	}

	return "", fmt.Errorf(
		"sing-box binary not found in ~/.swellbox/bin\n"+
			"Place %s next to Swell-Box once (offline package seeds the data dir), "+
			"or use tray → Update Core, or set SWELLBOX_CORE",
		name,
	)
}

func (m *Manager) Start() error {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return nil
	}

	bin, err := m.ResolveBinary()
	if err != nil {
		m.mu.Unlock()
		return err
	}
	if abs, err := filepath.Abs(bin); err == nil {
		bin = abs
	}
	// Always log which binary is started — users often have both the zip copy
	// and ~/.swellbox/bin; only one actually runs.
	log.Printf("starting core: %s", bin)
	if m.ConfigPath == "" {
		m.mu.Unlock()
		return fmt.Errorf("config path is empty")
	}
	if _, err := os.Stat(m.ConfigPath); err != nil {
		m.mu.Unlock()
		return fmt.Errorf("config: %w", err)
	}

	workDir := m.WorkDir
	if workDir == "" {
		workDir = filepath.Dir(m.ConfigPath)
	}

	// Validate config before starting (clearer errors than a silent crash).
	if err := CheckConfig(bin, m.ConfigPath, workDir); err != nil {
		m.mu.Unlock()
		return fmt.Errorf("config check failed: %w", err)
	}

	logDir, err := paths.LogsDir()
	if err != nil {
		m.mu.Unlock()
		return err
	}
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		m.mu.Unlock()
		return err
	}
	logPath := filepath.Join(logDir, "core.log")
	// Prevent unbounded growth: rotate when over maxLogBytes.
	rotateLogIfNeeded(logPath, maxLogBytes, keepLogBytes)

	// macOS TUN: start sing-box as root via authorization dialog (tray stays user).
	if m.NeedPrivileges && runtime.GOOS == "darwin" && os.Geteuid() != 0 {
		return m.startPrivileged(bin, m.ConfigPath, workDir, logPath)
	}

	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		m.mu.Unlock()
		return err
	}

	cmd := exec.Command(bin, "run", "-c", m.ConfigPath, "-D", workDir)
	cmd.Dir = workDir
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	hideWindow(cmd)

	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		m.mu.Unlock()
		return fmt.Errorf("start sing-box: %w", err)
	}
	// Windows: put child in Job Object so it dies with Swell-Box (no residual).
	if cmd.Process != nil {
		assignToJob(cmd.Process)
	}

	m.cmd = cmd
	m.logFile = logFile
	m.privileged = false
	m.privilegedPID = 0
	m.running = true
	m.mu.Unlock()

	// Reap child when it exits (do not double-lock while Start still holds mu).
	go func() {
		_ = cmd.Wait()
		m.mu.Lock()
		// Only clear if this is still the active process.
		if m.cmd == cmd {
			m.running = false
			m.cmd = nil
			if m.logFile != nil {
				_ = m.logFile.Close()
				m.logFile = nil
			}
		}
		m.mu.Unlock()
	}()

	// Brief settle; if process exits immediately, surface log hint.
	time.Sleep(300 * time.Millisecond)
	m.mu.Lock()
	running := m.running
	m.mu.Unlock()
	if !running {
		return fmt.Errorf("sing-box exited immediately; see %s", logPath)
	}
	return nil
}

func (m *Manager) Stop() error {
	m.mu.Lock()
	if m.privileged || m.privilegedPID > 0 {
		m.mu.Unlock()
		return m.stopPrivileged()
	}
	cmd := m.cmd
	if !m.running || cmd == nil || cmd.Process == nil {
		m.running = false
		m.cmd = nil
		m.mu.Unlock()
		return nil
	}
	// Detach from manager first so Wait goroutine won't race on log close.
	m.running = false
	m.cmd = nil
	logFile := m.logFile
	m.logFile = nil
	m.mu.Unlock()

	err := terminate(cmd)
	done := make(chan struct{})
	go func() {
		// Wait may already be consumed by the Start reaper goroutine; ignore error.
		_, _ = cmd.Process.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		_ = cmd.Process.Kill()
	}
	if logFile != nil {
		_ = logFile.Close()
	}
	return err
}

// TailLog returns the last portion of the core log for UI diagnostics.
func TailLog(max int64) (string, error) {
	logDir, err := paths.LogsDir()
	if err != nil {
		return "", err
	}
	path := filepath.Join(logDir, "core.log")
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return "", err
	}
	if max <= 0 {
		max = 4096
	}
	start := st.Size() - max
	if start < 0 {
		start = 0
	}
	if _, err := f.Seek(start, io.SeekStart); err != nil {
		return "", err
	}
	b, err := io.ReadAll(f)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
