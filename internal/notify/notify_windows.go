//go:build windows

package notify

import (
	"os"
	"path/filepath"
	"sync"

	"github.com/gen2brain/beeep"

	"github.com/swell-app/swellbox/internal/paths"
)

var (
	once    sync.Once
	iconPNG string
)

func initIcon() {
	once.Do(func() {
		// Prefer a small on-disk icon so Windows toast has an image.
		// Seed copies live under %USERPROFILE%\.swellbox if we write one; else empty.
		home, err := os.UserHomeDir()
		if err != nil {
			return
		}
		p := filepath.Join(home, ".swellbox", "icon.png")
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			iconPNG = p
		}
	})
}

func show(title, message string, isError bool) {
	initIcon()
	beeep.AppName = paths.AppName
	_ = beeep.Notify(title, message, iconPNG)
}
