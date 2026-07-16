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
		// Use the same color brand logo as macOS for a consistent look.
		home, err := os.UserHomeDir()
		if err != nil {
			return
		}
		for _, name := range []string{"logo.png", "icon.png"} {
			p := filepath.Join(home, ".swellbox", name)
			if st, err := os.Stat(p); err == nil && !st.IsDir() && st.Size() > 0 {
				iconPNG = p
				return
			}
		}
	})
}

func show(title, message string, isError bool) {
	initIcon()
	beeep.AppName = paths.AppName
	_ = beeep.Notify(title, message, iconPNG)
}
