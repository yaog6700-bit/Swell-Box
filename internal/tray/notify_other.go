//go:build !windows

package tray

func popup(title, message string) {
	// no-op on non-windows for now
}
