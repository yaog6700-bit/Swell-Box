package update

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/swell-app/swellbox/internal/paths"
)

// Official SagerNet rule-set URLs (binary .srs).
var geoFiles = []struct {
	Name string
	URL  string
}{
	{
		Name: "geosite-cn.srs",
		URL:  "https://raw.githubusercontent.com/SagerNet/sing-geosite/rule-set/geosite-cn.srs",
	},
	{
		Name: "geoip-cn.srs",
		URL:  "https://raw.githubusercontent.com/SagerNet/sing-geoip/rule-set/geoip-cn.srs",
	},
}

// UpdateGeoRules downloads latest geosite/geoip rule-sets into ~/.swellbox/rule-set/.
// Always overwrites existing files.
func UpdateGeoRules() error {
	home, err := paths.HomeDir()
	if err != nil {
		return err
	}
	dir := filepath.Join(home, "rule-set")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	for _, f := range geoFiles {
		// Reuse robust downloader (long timeout, retries, GitHub mirrors).
		tmp := filepath.Join(dir, f.Name+".tmp")
		if err := downloadFile(f.URL, tmp); err != nil {
			_ = os.Remove(tmp)
			return fmt.Errorf("%s: %w", f.Name, err)
		}
		// Guard against empty/corrupt payloads (downloadFile already checks >=1KB).
		if st, err := os.Stat(tmp); err != nil || st.Size() < 64 {
			_ = os.Remove(tmp)
			return fmt.Errorf("%s: file too small", f.Name)
		}
		dest := filepath.Join(dir, f.Name)
		_ = os.Remove(dest)
		if err := os.Rename(tmp, dest); err != nil {
			// Cross-device rename fallback
			data, rerr := os.ReadFile(tmp)
			_ = os.Remove(tmp)
			if rerr != nil {
				return fmt.Errorf("%s: %w", f.Name, err)
			}
			if werr := os.WriteFile(dest, data, 0o644); werr != nil {
				return fmt.Errorf("%s: %w", f.Name, werr)
			}
		}
	}
	return nil
}
