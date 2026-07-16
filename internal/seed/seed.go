package seed

import "embed"

//go:embed config.json icon.ico icon_off.ico icon.png icon_off.png icon_tun.ico icon_tun.png
//go:embed logo.png logo.ico
//go:embed rule-set/geosite-cn.srs rule-set/geoip-cn.srs
var files embed.FS

var (
	DefaultConfig = mustRead("config.json")
	// Monochrome tray glyphs (legacy / high-contrast). Prefer Logo* for brand parity
	// with original SingBoxClient (pickaxe).
	IconOnICO  = mustRead("icon.ico")
	IconOffICO = mustRead("icon_off.ico")
	IconOnPNG  = mustRead("icon.png")
	IconOffPNG = mustRead("icon_off.png")
	// IconTun* — compact pickaxe (fallback tray mark).
	IconTunICO = mustRead("icon_tun.ico")
	IconTunPNG = mustRead("icon_tun.png")
	// Logo* — full-color pickaxe brand (same family as original SingBoxClient tray).
	// Used for: menu-bar while running, macOS .app, desktop notifications.
	LogoPNG = mustRead("logo.png")
	LogoICO = mustRead("logo.ico")
	GeositeCNSRS = mustRead("rule-set/geosite-cn.srs")
	GeoipCNSRS   = mustRead("rule-set/geoip-cn.srs")
)

func mustRead(name string) []byte {
	b, err := files.ReadFile(name)
	if err != nil {
		panic("seed: " + err.Error())
	}
	return b
}
