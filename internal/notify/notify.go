package notify

// Info shows a non-blocking desktop notification.
func Info(title, message string) {
	go show(title, message, false)
}

// Error shows a non-blocking error notification.
func Error(title, message string) {
	go show(title, message, true)
}

// EnsurePermission pre-requests OS notification permission (macOS).
// Safe no-op on other platforms. Call once after UI is up so the first
// Start/Stop toast is not dropped while the permission dialog is pending.
func EnsurePermission() {
	ensurePermission()
}
