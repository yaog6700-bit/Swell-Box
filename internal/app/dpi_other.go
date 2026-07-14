//go:build !windows

package app

// EnableDPIAwareness is a no-op outside Windows.
func EnableDPIAwareness() {}
