package seed

import "embed"

//go:embed config.json icon.ico icon_off.ico icon.png icon_off.png
//go:embed rule-set/geosite-cn.srs rule-set/geoip-cn.srs
var files embed.FS

var (
	DefaultConfig = mustRead("config.json")
	IconOnICO     = mustRead("icon.ico")
	IconOffICO    = mustRead("icon_off.ico")
	IconOnPNG     = mustRead("icon.png")
	IconOffPNG    = mustRead("icon_off.png")
	GeositeCNSRS  = mustRead("rule-set/geosite-cn.srs")
	GeoipCNSRS    = mustRead("rule-set/geoip-cn.srs")
)

func mustRead(name string) []byte {
	b, err := files.ReadFile(name)
	if err != nil {
		panic("seed: " + err.Error())
	}
	return b
}
