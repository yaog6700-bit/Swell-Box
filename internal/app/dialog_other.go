//go:build !windows

package app

// ConfirmYesNo is not available without a GUI toolkit on non-Windows builds.
func ConfirmYesNo(title, body string) bool {
	return false
}

// AlertInfo is a no-op on non-Windows builds.
func AlertInfo(title, body string) {}
