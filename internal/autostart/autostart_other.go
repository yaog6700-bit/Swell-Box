//go:build !windows

package autostart

// Stub for non-Windows (can add macOS LaunchAgent later).

func Enabled() bool { return false }

func Enable() error { return nil }

func Disable() error { return nil }

func Set(on bool) error {
	if on {
		return Enable()
	}
	return Disable()
}
