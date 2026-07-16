//go:build !windows && !darwin

package app

// RelaunchElevated is not supported on this platform.
func RelaunchElevated() error {
	return errElevateUnsupported
}

var errElevateUnsupported = errStr("elevation restart is only supported on Windows")

type errStr string

func (e errStr) Error() string { return string(e) }
