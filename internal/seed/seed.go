package seed

import "embed"

//go:embed config.json icon.ico icon_off.ico icon.png icon_off.png icon_tun.ico icon_tun.png
//go:embed logo.png logo.ico
//go:embed rule-set/geosite-cn.srs rule-set/geoip-cn.srs
var files embed.FS

var (
	DefaultConfig = mustRead("config.json")
	// Tray: On = running, Off = stopped, Tun = TUN running (pickaxe + X).
	IconOnICO  = mustRead("icon.ico")
	IconOffICO = mustRead("icon_off.ico")
	IconOnPNG  = mustRead("icon.png")
	IconOffPNG = mustRead("icon_off.png")
	// IconTun* — pickaxe + X for TUN tray (Windows multi-size ICO + PNG).
	IconTunICO = mustRead("icon_tun.ico")
	IconTunPNG = mustRead("icon_tun.png")
	// Logo* — brand mark for notifications / packaging (color logo kept).
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
