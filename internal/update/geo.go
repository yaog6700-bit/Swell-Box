package update

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

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

	client := &http.Client{Timeout: 3 * time.Minute}
	for _, f := range geoFiles {
		if err := downloadGeo(client, f.URL, filepath.Join(dir, f.Name)); err != nil {
			return fmt.Errorf("%s: %w", f.Name, err)
		}
	}
	return nil
}

func downloadGeo(client *http.Client, url, dest string) error {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "Swell-Box/"+AppVersion)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("http %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20)) // 32MB max
	if err != nil {
		return err
	}
	if len(data) < 64 {
		return fmt.Errorf("file too small (%d bytes)", len(data))
	}
	tmp := dest + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	_ = os.Remove(dest)
	return os.Rename(tmp, dest)
}
