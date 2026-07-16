package main

import (
	"log"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/swell-app/swellbox/internal/app"
	"github.com/swell-app/swellbox/internal/config"
	"github.com/swell-app/swellbox/internal/core"
	"github.com/swell-app/swellbox/internal/notify"
	"github.com/swell-app/swellbox/internal/paths"
	"github.com/swell-app/swellbox/internal/seed"
	"github.com/swell-app/swellbox/internal/tray"
	"github.com/swell-app/swellbox/internal/update"
)

func main() {
	runtime.LockOSThread()
	// Windows HiDPI: must run before any UI so tray menus render crisp (not bitmap-scaled).
	app.EnableDPIAwareness()

	// Single instance: second launch exits (no second tray icon / port fight).
	ok, err := app.AcquireSingleInstance()
	if err != nil {
		log.Fatal("single instance: ", err)
	}
	if !ok {
		// Non-blocking tip then exit so we don't leave a second process waiting on a dialog.
		notify.Info(paths.AppName, "已在运行（只能开一个），请查看系统托盘。")
		time.Sleep(400 * time.Millisecond)
		os.Exit(0)
	}
	defer app.ReleaseSingleInstance()
	defer core.CloseJob()

	if err := app.BootstrapDataDir(seed.DefaultConfig, seed.IconOnPNG, seed.LogoPNG, app.RuleSetFiles{
		GeositeCN: seed.GeositeCNSRS,
		GeoipCN:   seed.GeoipCNSRS,
	}); err != nil {
		log.Fatal("bootstrap: ", err)
	}

	settings, err := config.LoadAppSettings()
	if err != nil {
		log.Fatal("app settings: ", err)
	}

	// Tray icons — same three-state logic on Windows and macOS:
	//   stopped  → Off (pickaxe + X)
	//   running  → On  (pickaxe)
	//   TUN      → Tun (pickaxe + X)
	// Windows uses .ico; macOS/Linux use .png (template icon on mac menu bar).
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
