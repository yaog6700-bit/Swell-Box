//go:build !windows

package notify

import (
	"log"
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
		home, err := os.UserHomeDir()
		if err != nil {
			return
		}
		// Prefer color brand logo (same art as macOS .app AppIcon).
		// Fall back to monochrome tray glyph if logo missing.
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
	if err := beeep.Notify(title, message, iconPNG); err != nil {
		if isError {
			log.Printf("[notify:error] %s: %s", title, message)
			return
		}
		log.Printf("[notify] %s: %s", title, message)
	}
}
