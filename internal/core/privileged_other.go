//go:build !darwin

package core

import "fmt"

func (m *Manager) startPrivileged(bin, configPath, workDir, logPath string) error {
	m.mu.Unlock()
	return fmt.Errorf("privileged core start is only implemented on macOS; run as administrator/root or use system proxy")
}

func (m *Manager) stopPrivileged() error {
	m.mu.Lock()
	m.running = false
	m.privileged = false
	m.privilegedPID = 0
	m.cmd = nil
	m.mu.Unlock()
	return nil
}
