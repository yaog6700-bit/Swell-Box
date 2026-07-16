//go:build windows

package tray

import "github.com/swell-app/swellbox/internal/app"

// popup shows a modal dialog near the system tray (above the taskbar),
// instead of a screen-centered box that covers the tray icon.
func popup(title, message string) {
	app.AlertInfo(title, message)
}
