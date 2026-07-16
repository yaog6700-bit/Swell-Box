//go:build !windows

package app

// RelaunchElevated is Windows-only (UAC runas).
func RelaunchElevated() error {
	return errElevateUnsupported
}

var errElevateUnsupported = errStr("elevation restart is only supported on Windows")

type errStr string

func (e errStr) Error() string { return string(e) }
