package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/swell-app/swellbox/internal/paths"
)

// SwellTunTag is the runtime-injected TUN inbound tag (not written to user config).
const SwellTunTag = "swell-tun"

// PrepareRuntimeConfig reads the user config, injects / normalizes the official
// API + dashboard service, and writes a runtime file the core process will load.
//
// User configs stay untouched; only the generated runtime copy is modified.
// When tunMode is true, a TUN inbound is injected unless the user config already
// has a tun inbound.
func PrepareRuntimeConfig(userConfigPath, runtimePath string, dashboardPort int, tunMode bool) error {
	raw, err := os.ReadFile(userConfigPath)
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}

	var root map[string]any
	if err := json.Unmarshal(raw, &root); err != nil {
		return fmt.Errorf("parse config JSON: %w", err)
	}

	if dashboardPort <= 0 {
		dashboardPort = paths.DefaultPort
	}
	ensureAPIService(root, dashboardPort)
	ensureClashAPI(root, "127.0.0.1:9090")
	ensureCacheFile(root)
	// Prefer local rule-set files under workdir (offline-first).
	preferLocalRuleSets(root)
	// sing-box 鈮?.12 rejects detour:"direct" on DNS servers.
	stripDirectDNSDetour(root)
	applyTunMode(root, tunMode)

	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(runtimePath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(runtimePath, out, 0o644)
}

// applyTunMode injects or removes the managed TUN inbound on the runtime config.
func applyTunMode(root map[string]any, enabled bool) {
	// Always drop our previous injection first (idempotent rebuild).
	inbounds, _ := root["inbounds"].([]any)
	var kept []any
	for _, item := range inbounds {
		m, ok := item.(map[string]any)
		if !ok {
			kept = append(kept, item)
			continue
		}
		if tag, _ := m["tag"].(string); tag == SwellTunTag {
			continue
		}
		kept = append(kept, item)
	}
	inbounds = kept

	if enabled && !hasUserTun(inbounds) {
		inbounds = append(inbounds, map[string]any{
			"type":           "tun",
			"tag":            SwellTunTag,
			"interface_name": "swell-tun",
			"address":        []any{"172.19.0.1/30"},
			"mtu":            9000,
			"auto_route":     true,
			"strict_route":   true,
			"stack":          "mixed",
		})
		// Avoid routing loops when TUN takes over the default route.
		route, _ := root["route"].(map[string]any)
		if route == nil {
			route = map[string]any{}
		}
		route["auto_detect_interface"] = true
		root["route"] = route
	}
	root["inbounds"] = inbounds
}

func hasUserTun(inbounds []any) bool {
	for _, item := range inbounds {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if t, _ := m["type"].(string); t == "tun" {
			return true
		}
	}
	return false
}

func ensureAPIService(root map[string]any, port int) {
	services, _ := root["services"].([]any)
	var found bool
	for i, item := range services {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		t, _ := m["type"].(string)
		if t != "api" {
			continue
		}
		found = true
		if _, ok := m["listen"]; !ok {
			m["listen"] = "127.0.0.1"
		}
		m["listen_port"] = port
		dash, _ := m["dashboard"].(map[string]any)
		if dash == nil {
			m["dashboard"] = map[string]any{"enabled": true}
		} else {
			dash["enabled"] = true
			m["dashboard"] = dash
		}
		services[i] = m
	}
	if !found {
		services = append(services, map[string]any{
			"type":        "api",
			"tag":         "api",
			"listen":      "127.0.0.1",
			"listen_port": port,
			"dashboard": map[string]any{
				"enabled": true,
			},
		})
	}
	root["services"] = services
}

// ensureClashAPI injects experimental.clash_api for tray node switching.
func ensureClashAPI(root map[string]any, addr string) {
	exp, _ := root["experimental"].(map[string]any)
	if exp == nil {
		exp = map[string]any{}
	}
	clash, _ := exp["clash_api"].(map[string]any)
	if clash == nil {
		clash = map[string]any{}
	}
	if _, ok := clash["external_controller"]; !ok || clash["external_controller"] == "" {
		clash["external_controller"] = addr
	}
	exp["clash_api"] = clash
	root["experimental"] = exp
}

func ensureCacheFile(root map[string]any) {
	exp, _ := root["experimental"].(map[string]any)
	if exp == nil {
		exp = map[string]any{}
	}
	cf, _ := exp["cache_file"].(map[string]any)
	if cf == nil {
		cf = map[string]any{}
	}
	if cf["enabled"] == nil {
		cf["enabled"] = true
	}
	if cf["path"] == nil || cf["path"] == "" {
		cf["path"] = "cache.db"
	}
	// store selector choice across restarts
	if cf["store_selected"] == nil {
		// newer sing-box uses cache_file only; keep path
	}
	exp["cache_file"] = cf
	root["experimental"] = exp
}

// stripDirectDNSDetour removes detour:"direct" from DNS servers.
// Newer sing-box: "detour to an empty direct outbound makes no sense".
func stripDirectDNSDetour(root map[string]any) {
	dns, _ := root["dns"].(map[string]any)
	if dns == nil {
		return
	}
	servers, _ := dns["servers"].([]any)
	for i, item := range servers {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if d, _ := m["detour"].(string); d == "direct" {
			delete(m, "detour")
			servers[i] = m
		}
	}
	dns["servers"] = servers
	root["dns"] = dns
}

// preferLocalRuleSets rewrites known remote CN rule-sets to bundled local paths
// when the files exist under ~/.swellbox/rule-set/.
func preferLocalRuleSets(root map[string]any) {
	route, _ := root["route"].(map[string]any)
	if route == nil {
		return
	}
	list, _ := route["rule_set"].([]any)
	if len(list) == 0 {
		return
	}
	home, err := paths.HomeDir()
	if err != nil {
		return
	}
	localMap := map[string]string{
		"geosite-cn": "rule-set/geosite-cn.srs",
		"geoip-cn":   "rule-set/geoip-cn.srs",
	}
	for i, item := range list {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		tag, _ := m["tag"].(string)
		rel, ok := localMap[tag]
		if !ok {
			continue
		}
		full := filepath.Join(home, filepath.FromSlash(rel))
		if st, err := os.Stat(full); err != nil || st.IsDir() || st.Size() == 0 {
			continue
		}
		m["type"] = "local"
		m["format"] = "binary"
		m["path"] = rel
		delete(m, "url")
		delete(m, "download_detour")
		delete(m, "update_interval")
		list[i] = m
	}
	route["rule_set"] = list
	root["route"] = route
}

// RuntimeConfigPath returns ~/.swellbox/runtime/config.runtime.json
func RuntimeConfigPath() (string, error) {
	dir, err := paths.HomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "runtime", "config.runtime.json"), nil
}
