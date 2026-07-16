//go:build !windows

package app

// ConfirmYesNo is not available without a GUI toolkit on non-Windows builds.
func ConfirmYesNo(title, body string) bool {
	return false
}

// RelaunchElevated is Windows-only (UAC runas).
func RelaunchElevated() error {
	return errElevateUnsupported
}

var errElevateUnsupported = errStr("elevation restart is only supported on Windows")

type errStr string

func (e errStr) Error() string { return string(e) }
