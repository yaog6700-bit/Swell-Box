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
		// Toast icon: monochrome pickaxe (same as process / tray On), not color logo.
		home, err := os.UserHomeDir()
		if err != nil {
			return
		}
		for _, name := range []string{"icon.png", "logo.png"} {
			p := filepath.Join(home, ".swellbox", name)
			if st, err := os.Stat(p); err == nil && !st.IsDir() && st.Size() > 0 {
				iconPNG = p
				return
			}
		}
	})
}

func ensurePermission() {}

func show(title, message string, isError bool) {
	initIcon()
	beeep.AppName = paths.AppName
	_ = beeep.Notify(title, message, iconPNG)
}
