package app

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/swell-app/swellbox/internal/paths"
)

// RuleSetFiles holds embedded geosite/geoip rule-set binaries.
type RuleSetFiles struct {
	GeositeCN []byte
	GeoipCN   []byte
}

// BootstrapDataDir ensures ~/.swellbox exists, seeds default config once,
// and always installs local rule-set files needed for offline-first routing.
//
// Call InstallBundledCore separately (or via update.EnsureCore) to pick up
// sing-box.exe shipped next to Swell-Box.exe.
//
// iconPNG is the monochrome app mark (tray On / notifications / process look).
// logoPNG is optional color brand art kept on disk for packaging reference.
func BootstrapDataDir(defaultConfig []byte, iconPNG []byte, logoPNG []byte, rules RuleSetFiles) error {
	home, err := paths.HomeDir()
	if err != nil {
		return err
	}
	for _, sub := range []string{"", "bin", "logs", "runtime", "rule-set"} {
		p := home
		if sub != "" {
			p = filepath.Join(home, sub)
		}
		if err := os.MkdirAll(p, 0o755); err != nil {
			return err
		}
	}

	// Always refresh monochrome icon so toasts match the current build.
	if len(iconPNG) > 0 {
		_ = os.WriteFile(filepath.Join(home, "icon.png"), iconPNG, 0o644)
	}
	if len(logoPNG) > 0 {
		_ = os.WriteFile(filepath.Join(home, "logo.png"), logoPNG, 0o644)
	}

	// Always ensure local rule-sets exist (offline first start).
	rsDir := filepath.Join(home, "rule-set")
	if len(rules.GeositeCN) > 0 {
		if err := writeIfMissingOrEmpty(filepath.Join(rsDir, "geosite-cn.srs"), rules.GeositeCN); err != nil {
			return err
		}
	}
	if len(rules.GeoipCN) > 0 {
		if err := writeIfMissingOrEmpty(filepath.Join(rsDir, "geoip-cn.srs"), rules.GeoipCN); err != nil {
			return err
		}
	}

	dst := filepath.Join(home, "config.json")
	if _, err := os.Stat(dst); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	if len(defaultConfig) == 0 {
		return fmt.Errorf("empty default config")
	}
	return os.WriteFile(dst, defaultConfig, 0o644)
}

func writeIfMissingOrEmpty(path string, data []byte) error {
	st, err := os.Stat(path)
	if err == nil && st.Size() > 0 {
		return nil
	}
	return os.WriteFile(path, data, 0o644)
}
