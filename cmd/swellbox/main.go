package main

import (
	"log"
	"os"
	"path/filepath"
	"runtime"

	"github.com/swell-app/swellbox/internal/app"
	"github.com/swell-app/swellbox/internal/config"
	"github.com/swell-app/swellbox/internal/core"
	"github.com/swell-app/swellbox/internal/paths"
	"github.com/swell-app/swellbox/internal/seed"
	"github.com/swell-app/swellbox/internal/tray"
	"github.com/swell-app/swellbox/internal/update"
)

func main() {
	runtime.LockOSThread()
	// Windows HiDPI: must run before any UI so tray menus render crisp (not bitmap-scaled).
	app.EnableDPIAwareness()

	if err := app.BootstrapDataDir(seed.DefaultConfig, seed.IconOnPNG, app.RuleSetFiles{
		GeositeCN: seed.GeositeCNSRS,
		GeoipCN:   seed.GeoipCNSRS,
	}); err != nil {
		log.Fatal("bootstrap: ", err)
	}

	settings, err := config.LoadAppSettings()
	if err != nil {
		log.Fatal("app settings: ", err)
	}

	icons := tray.Icons{}
	if runtime.GOOS == "windows" {
		icons.On = seed.IconOnICO
		icons.Off = seed.IconOffICO
		icons.Tun = seed.IconTunICO
	} else {
		icons.On = seed.IconOnPNG
		icons.Off = seed.IconOffPNG
		icons.Tun = seed.IconTunPNG
	}

	// Optional: allow SWELLBOX_CORE to override core path for dev.
	if p := os.Getenv("SWELLBOX_CORE"); p != "" {
		if abs, err := filepath.Abs(p); err == nil {
			settings.CorePath = abs
		} else {
			settings.CorePath = p
		}
	}

	log.Printf("%s %s ready — data under home/.%s", paths.AppName, update.AppVersion, paths.AppID)

	ctrl := &tray.Controller{
		Icons: icons,
		Core:  &core.Manager{},
		App:   settings,
	}
	ctrl.Run()
}
