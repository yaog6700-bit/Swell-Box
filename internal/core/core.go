package core

import (
	"fmt"
	"io"
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

// ResolveBinary finds the sing-box executable.
// Order must match update.resolveCoreBin so "check core update" and the
// running process use the same file:
//
//	explicit CorePath → ~/.swellbox/bin (in-app updates) → next to app → PATH
//
// Previously "next to app" was preferred, so a successful tray update wrote
// a new binary under ~/.swellbox/bin while Start() kept launching the older
// copy shipped next to Swell-Box.exe — Dashboard still showed the old version.
func (m *Manager) ResolveBinary() (string, error) {
	name := paths.CoreBinaryName()

	if m.CorePath != "" {
		if st, err := os.Stat(m.CorePath); err == nil && !st.IsDir() {
			return m.CorePath, nil
		}
		return "", fmt.Errorf("core_path not found: %s", m.CorePath)
	}

	// 1) User data bin — destination of "Update Core" / EnsureCore installs.
	if binDir, err := paths.BinDir(); err == nil {
		candidate := filepath.Join(binDir, name)
		if st, err := os.Stat(candidate); err == nil && !st.IsDir() && st.Size() > 0 {
			return candidate, nil
		}
	}

	// 2) Next to the Swell-Box executable (incl. macOS .app layouts / full.zip).
	if exePath, err := os.Executable(); err == nil {
		if r, err := filepath.EvalSymlinks(exePath); err == nil {
			exePath = r
		}
		dir := filepath.Dir(exePath)
		for _, candidate := range []string{
			filepath.Join(dir, name),
			filepath.Join(dir, "bin", name),
			filepath.Join(dir, "core", name),
			filepath.Clean(filepath.Join(dir, "..", "Resources", name)),
			filepath.Clean(filepath.Join(dir, "..", "..", "..", name)),
		} {
			if st, err := os.Stat(candidate); err == nil && !st.IsDir() && st.Size() > 0 {
				return candidate, nil
			}
		}
	}

	// 3) PATH
	if p, err := exec.LookPath(name); err == nil {
		return p, nil
	}
	// Also try without .exe on Windows PATH quirks
	if runtime.GOOS == "windows" {
		if p, err := exec.LookPath("sing-box"); err == nil {
			return p, nil
		}
	}

	return "", fmt.Errorf(
		"sing-box binary not found\n"+
			"Put %s next to Swell-Box, or in ~/.swellbox/bin (Windows: %%USERPROFILE%%\\.swellbox\\bin), or on PATH",
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
