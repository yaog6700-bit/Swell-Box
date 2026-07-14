package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/swell-app/swellbox/internal/paths"
)

// AppSettings is SWELL Box client state (not sing-box core config).
type AppSettings struct {
	// ActiveConfig is the active config file name under the data dir, e.g. config.json
	ActiveConfig string `json:"active_config"`
	// CorePath is an optional absolute path to the sing-box binary.
	// Empty means auto-detect (next to app, PATH, or ~/.swellbox/bin).
	CorePath string `json:"core_path,omitempty"`
	// DashboardPort is the injected API / dashboard port (default 9091).
	DashboardPort int `json:"dashboard_port"`
	// AutoStartProxy starts the core when the tray app launches.
	AutoStartProxy bool `json:"auto_start_proxy"`
	// SystemProxy enables Windows system proxy when core is running.
	SystemProxy bool `json:"system_proxy"`
	// TunMode injects a TUN inbound into the runtime config (global capture).
	// Prefer admin/root when enabling; mutually exclusive with SystemProxy in the tray UI.
	TunMode bool `json:"tun_mode"`
	// CoreChannel is "stable" or "pre" for updates / first-run download.
	CoreChannel string `json:"core_channel"`
	// Language is "zh" or "en". Default zh.
	Language string `json:"language"`
}

func DefaultAppSettings() *AppSettings {
	return &AppSettings{
		ActiveConfig:  "config.json",
		DashboardPort: paths.DefaultPort,
		// pre: official dashboard (api service) needs 1.14+ for now
		CoreChannel: "pre",
		Language:    "zh",
		// System proxy on by default when user starts (can turn off in tray)
		SystemProxy: true,
	}
}

func LoadAppSettings() (*AppSettings, error) {
	path, err := paths.AppConfigPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			s := DefaultAppSettings()
			_ = SaveAppSettings(s)
			return s, nil
		}
		return nil, err
	}
	s := DefaultAppSettings()
	if err := json.Unmarshal(data, s); err != nil {
		return nil, err
	}
	if s.ActiveConfig == "" {
		s.ActiveConfig = "config.json"
	}
	if s.DashboardPort <= 0 {
		s.DashboardPort = paths.DefaultPort
	}
	if s.Language != "en" && s.Language != "zh" {
		s.Language = "zh"
	}
	if s.CoreChannel != "stable" && s.CoreChannel != "pre" {
		s.CoreChannel = "pre"
	}
	return s, nil
}

func SaveAppSettings(s *AppSettings) error {
	path, err := paths.AppConfigPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// ListConfigFiles returns config*.json names in the data directory.
func ListConfigFiles() ([]string, error) {
	dir, err := paths.ConfigDir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if name == "app.json" {
			continue
		}
		if strings.HasPrefix(name, "config") && strings.HasSuffix(name, ".json") {
			names = append(names, name)
		}
	}
	return names, nil
}

func ActiveConfigPath(s *AppSettings) (string, error) {
	dir, err := paths.ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, s.ActiveConfig), nil
}
