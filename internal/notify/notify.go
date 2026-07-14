package notify

// Info shows a non-blocking desktop notification.
func Info(title, message string) {
	go show(title, message, false)
}

// Error shows a non-blocking error notification.
func Error(title, message string) {
	go show(title, message, true)
}
