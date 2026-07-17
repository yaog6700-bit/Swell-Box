//go:build !windows && !darwin

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

func ensurePermission() {}

func initIcon() {
	once.Do(func() {
		home, err := os.UserHomeDir()
		if err != nil {
			return
		}
		// Prefer monochrome app mark (same as .app / process icon), then color logo.
		for _, name := range []string{"icon.png", "logo.png"} {
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
