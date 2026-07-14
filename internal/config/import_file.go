package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/swell-app/swellbox/internal/paths"
)

var safeName = regexp.MustCompile(`[^a-zA-Z0-9._\-\p{L}]+`)

// ImportConfigFile copies a user-selected JSON into the data dir as config*.json,
// validates it is JSON object, sets it active, and returns the saved file name.
func ImportConfigFile(settings *AppSettings, srcPath string) (savedName string, err error) {
	srcPath = strings.TrimSpace(srcPath)
	if srcPath == "" {
		return "", fmt.Errorf("empty path")
	}
	raw, err := os.ReadFile(srcPath)
	if err != nil {
		return "", fmt.Errorf("read: %w", err)
	}
	var root map[string]any
	if err := json.Unmarshal(raw, &root); err != nil {
		return "", fmt.Errorf("not valid JSON: %w", err)
	}
	// Light sanity: sing-box configs are objects; prefer having outbounds or inbounds.
	if len(root) == 0 {
		return "", fmt.Errorf("empty config object")
	}

	dir, err := paths.ConfigDir()
	if err != nil {
		return "", err
	}
	base := filepath.Base(srcPath)
	base = strings.TrimSuffix(base, filepath.Ext(base))
	base = safeName.ReplaceAllString(base, "_")
	base = strings.Trim(base, "._-")
	if base == "" || strings.EqualFold(base, "app") {
		base = "imported"
	}
	// Always store as config.<name>.json so it shows in Configs menu.
	name := "config." + base + ".json"
	if name == "config.config.json" {
		name = "config.imported.json"
	}
	dst := filepath.Join(dir, name)
	// Avoid clobber without suffix
	if _, err := os.Stat(dst); err == nil {
		for i := 2; i < 100; i++ {
			cand := fmt.Sprintf("config.%s-%d.json", base, i)
			p := filepath.Join(dir, cand)
			if _, err := os.Stat(p); os.IsNotExist(err) {
				name = cand
				dst = p
				break
			}
		}
	}

	// Pretty-print for readability
	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(dst, out, 0o644); err != nil {
		return "", err
	}

	settings.ActiveConfig = name
	if err := SaveAppSettings(settings); err != nil {
		return name, err
	}
	return name, nil
}
