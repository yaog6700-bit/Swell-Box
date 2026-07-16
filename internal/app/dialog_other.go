//go:build !windows && !darwin

package app

// ConfirmYesNo is not available without a GUI toolkit on this platform.
func ConfirmYesNo(title, body string) bool {
	return false
}

// AlertInfo is a no-op on this platform.
func AlertInfo(title, body string) {}
