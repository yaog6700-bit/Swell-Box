package config

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/swell-app/swellbox/internal/paths"
	"gopkg.in/yaml.v3"
)

// shoesGroupKey returns "type" for shoes >= 0.3.5 (default/modern), or "strategy" for shoes <= 0.3.4.
func shoesGroupKey() string {
	name := paths.CoreBinaryNameFor(paths.EngineShoes)
	var candidates []string
	if binDir, err := paths.BinDir(); err == nil && binDir != "" {
		candidates = append(candidates, filepath.Join(binDir, name))
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		candidates = append(candidates, filepath.Join(home, "Desktop", name))
	}
	if p, err := exec.LookPath(name); err == nil {
		candidates = append(candidates, p)
	}

	for _, cand := range candidates {
		if st, err := os.Stat(cand); err == nil && !st.IsDir() && st.Size() > 0 {
			out, err := exec.Command(cand, "version").Output()
			if err != nil {
				out, _ = exec.Command(cand, "--version").Output()
			}
			s := strings.TrimSpace(string(out))
			if s != "" {
				// Shoes <= 0.3.4 used "strategy: selector"
				if strings.Contains(s, "0.3.4") || strings.Contains(s, "0.3.3") ||
					strings.Contains(s, "0.3.2") || strings.Contains(s, "0.3.1") ||
					strings.Contains(s, "0.3.0") {
					return "strategy"
				}
				// Shoes >= 0.3.5 strictly requires "type: selector" (PR #221)
				return "type"
			}
		}
	}
	// Default to modern shoes (>= 0.3.5)
	return "type"
}

// PrepareRuntimeShoesConfig prepares the YAML configuration file for the shoes core.
// It translates node definitions from sing-box JSON configuration into shoes YAML,
// or uses the customShoesPath if specified.
func PrepareRuntimeShoesConfig(userConfigPath, runtimePath, customShoesPath string, clashPort, mixedPort int, secret string) error {
	if customShoesPath != "" {
		if raw, err := os.ReadFile(customShoesPath); err == nil && len(raw) > 0 {
			if err := os.MkdirAll(filepath.Dir(runtimePath), 0o755); err != nil {
				return err
			}
			return os.WriteFile(runtimePath, raw, 0o644)
		}
	}

	raw, err := os.ReadFile(userConfigPath)
	if err != nil {
		return fmt.Errorf("read user config: %w", err)
	}

	var root map[string]any
	if err := json.Unmarshal(raw, &root); err != nil {
		return fmt.Errorf("parse config JSON: %w", err)
	}

	if clashPort <= 0 {
		clashPort = 9090
	}
	if mixedPort <= 0 {
		mixedPort = 7890
	}
	if secret == "" {
		secret = "swellbox"
	}

	outbounds, _ := root["outbounds"].([]any)
	byTag := make(map[string]map[string]any)
	var selectorDefault string
	var selectorMembers []string

	for _, item := range outbounds {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		typ, _ := m["type"].(string)
		tag, _ := m["tag"].(string)
		if tag == "" {
			continue
		}
		if typ == "selector" {
			if selectorDefault == "" {
				selectorDefault, _ = m["default"].(string)
				for _, o := range toStringSlice(m["outbounds"]) {
					if o != "direct" && o != "block" && o != "dns" {
						selectorMembers = append(selectorMembers, o)
					}
				}
			}
			continue
		}
		byTag[tag] = m
	}

	var clientProxies []any
	for _, tag := range selectorMembers {
		ob, ok := byTag[tag]
		if !ok {
			continue
		}
		proxy := translateOutboundToShoes(tag, ob)
		if proxy != nil {
			clientProxies = append(clientProxies, proxy)
		}
	}

	// If no selector members translated, check all outbounds
	if len(clientProxies) == 0 {
		for tag, ob := range byTag {
			proxy := translateOutboundToShoes(tag, ob)
			if proxy != nil {
				clientProxies = append(clientProxies, proxy)
			}
		}
	}

	// Fallback if still empty: add direct proxy
	if len(clientProxies) == 0 {
		clientProxies = append(clientProxies, map[string]any{
			"name":    "direct",
			"address": "127.0.0.1:0",
			"protocol": map[string]any{
				"type": "socks",
			},
		})
		selectorDefault = "direct"
	} else {
		// Ensure selectorDefault points to an existing proxy in clientProxies
		defaultValid := false
		for _, p := range clientProxies {
			if pm, ok := p.(map[string]any); ok {
				if n, _ := pm["name"].(string); n == selectorDefault {
					defaultValid = true
					break
				}
			}
		}
		if !defaultValid {
			first := clientProxies[0].(map[string]any)
			selectorDefault, _ = first["name"].(string)
		}
	}

	var yamlDoc []any

	// 1. Clash API controller
	yamlDoc = append(yamlDoc, map[string]any{
		"clash_api_address: ": nil, // placeholder to control key order if needed
	})
	clashApiEntry := map[string]any{
		"clash_api_address": fmt.Sprintf("127.0.0.1:%d", clashPort),
		"secret":            secret,
		"cors_origins": []string{
			"*",
		},
	}
	yamlDoc[0] = clashApiEntry

	// 2. Selector Client Group
	// Shoes 0.3.5+ strictly uses "type: selector"; shoes <= 0.3.4 used "strategy: selector".
	groupKey := shoesGroupKey()
	selectorGroup := map[string]any{
		"client_group":   "proxy",
		groupKey:         "selector",
		"default":        selectorDefault,
		"client_proxies": clientProxies,
	}
	yamlDoc = append(yamlDoc, selectorGroup)

	// 3. Mixed Inbound Server
	mixedInbound := map[string]any{
		"name":    "mixed-in",
		"address": fmt.Sprintf("127.0.0.1:%d", mixedPort),
		"sniff":   true,
		"protocol": map[string]any{
			"type":        "mixed",
			"udp_enabled": true,
		},
		"rules": buildShoesRules(root),
	}
	yamlDoc = append(yamlDoc, mixedInbound)

	data, err := yaml.Marshal(yamlDoc)
	if err != nil {
		return fmt.Errorf("marshal shoes YAML: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(runtimePath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(runtimePath, data, 0o644)
}

func buildShoesRules(root map[string]any) []any {
	var rules []any

	// 1. Private IP rule (direct, no_resolve)
	rules = append(rules, map[string]any{
		"no_resolve": true,
		"masks": map[string]any{
			"any": []any{
				map[string]any{
					"ip_cidr": []string{
						"10.0.0.0/8",
						"172.16.0.0/12",
						"192.168.0.0/16",
						"127.0.0.0/8",
						"fc00::/7",
					},
				},
			},
		},
		"client_chain": "direct",
	})

	// Rule sets lookup table from root["route"]["rule_set"]
	ruleSetMap := make(map[string]string)
	if route, ok := root["route"].(map[string]any); ok {
		if rsList, ok := route["rule_set"].([]any); ok {
			for _, item := range rsList {
				if rsm, ok := item.(map[string]any); ok {
					tag, _ := rsm["tag"].(string)
					path, _ := rsm["path"].(string)
					if tag != "" && path != "" {
						ruleSetMap[tag] = path
					}
				}
			}
		}
	}

	// Swell-Box default rule-sets: geosite-cn and geoip-cn
	home, _ := paths.HomeDir()
	geositeCN := filepath.Join(home, "rule-set", "geosite-cn.srs")
	geoipCN := filepath.Join(home, "rule-set", "geoip-cn.srs")

	var cnMasks []any
	if st, err := os.Stat(geositeCN); err == nil && !st.IsDir() && st.Size() > 0 {
		cnMasks = append(cnMasks, map[string]any{"path": filepath.ToSlash(geositeCN)})
	}
	if st, err := os.Stat(geoipCN); err == nil && !st.IsDir() && st.Size() > 0 {
		cnMasks = append(cnMasks, map[string]any{"path": filepath.ToSlash(geoipCN)})
	}
	// Always include .cn domain suffix for Chinese sites
	cnMasks = append(cnMasks, map[string]any{"domain_suffix": []string{"cn"}})

	if len(cnMasks) > 0 {
		rules = append(rules, map[string]any{
			"masks": map[string]any{
				"any": cnMasks,
			},
			"client_chain": "direct",
		})
	}

	// 3. Translate custom route rules if present
	if route, ok := root["route"].(map[string]any); ok {
		if rList, ok := route["rules"].([]any); ok {
			for _, item := range rList {
				rm, ok := item.(map[string]any)
				if !ok {
					continue
				}
				action, _ := rm["action"].(string)
				if action == "sniff" || action == "hijack-dns" {
					continue
				}
				if boolVal(rm["ip_is_private"]) {
					continue // already covered in rule 1
				}

				outbound, _ := rm["outbound"].(string)
				chain := "proxy"
				if outbound == "direct" {
					chain = "direct"
				} else if outbound == "block" || action == "reject" {
					chain = "block"
				}

				var masks []any
				if ds := toStringSlice(rm["domain_suffix"]); len(ds) > 0 {
					masks = append(masks, map[string]any{"domain_suffix": ds})
				}
				if dk := toStringSlice(rm["domain_keyword"]); len(dk) > 0 {
					masks = append(masks, map[string]any{"domain_keyword": dk})
				}
				if cidrs := toStringSlice(rm["ip_cidr"]); len(cidrs) > 0 {
					masks = append(masks, map[string]any{"ip_cidr": cidrs})
				}
				if rsTag, ok := rm["rule_set"].(string); ok && rsTag != "" {
					if rsTag != "geosite-cn" && rsTag != "geoip-cn" {
						if p, ok := ruleSetMap[rsTag]; ok {
							if !filepath.IsAbs(p) && home != "" {
								p = filepath.Join(home, p)
							}
							if st, err := os.Stat(p); err == nil && !st.IsDir() {
								masks = append(masks, map[string]any{"path": filepath.ToSlash(p)})
							}
						}
					}
				}

				if len(masks) == 1 {
					ruleObj := map[string]any{
						"masks": masks[0],
					}
					if chain == "block" {
						ruleObj["action"] = "block"
					} else {
						ruleObj["client_chain"] = chain
					}
					rules = append(rules, ruleObj)
				} else if len(masks) > 1 {
					ruleObj := map[string]any{
						"masks": map[string]any{"any": masks},
					}
					if chain == "block" {
						ruleObj["action"] = "block"
					} else {
						ruleObj["client_chain"] = chain
					}
					rules = append(rules, ruleObj)
				}
			}
		}
	}

	// 4. Final rule: everything else -> proxy
	rules = append(rules, map[string]any{
		"masks":        "0.0.0.0/0",
		"client_chain": "proxy",
	})

	return rules
}

func translateOutboundToShoes(tag string, ob map[string]any) map[string]any {
	typ, _ := ob["type"].(string)
	server, _ := ob["server"].(string)
	port := anyIntVal(ob["server_port"])
	if server == "" || port <= 0 {
		return nil
	}
	addr := fmt.Sprintf("%s:%d", server, port)
	if strings.Contains(server, ":") && !strings.HasPrefix(server, "[") {
		addr = fmt.Sprintf("[%s]:%d", server, port)
	}

	switch typ {
	case "shadowsocks":
		method, _ := ob["method"].(string)
		password, _ := ob["password"].(string)
		if method == "" || password == "" {
			return nil
		}
		return map[string]any{
			"name":    tag,
			"address": addr,
			"protocol": map[string]any{
				"type":     "shadowsocks",
				"cipher":   method,
				"password": password,
			},
		}

	case "vless":
		uuid, _ := ob["uuid"].(string)
		flow, _ := ob["flow"].(string)
		tlsMap, _ := ob["tls"].(map[string]any)

		vlessProto := map[string]any{
			"type":    "vless",
			"user_id": uuid,
		}

		if tlsMap != nil && boolVal(tlsMap["enabled"]) {
			sni, _ := tlsMap["server_name"].(string)
			if sni == "" {
				sni = server
			}
			insecure := boolVal(tlsMap["insecure"])

			if reality, ok := tlsMap["reality"].(map[string]any); ok && boolVal(reality["enabled"]) {
				pbk, _ := reality["public_key"].(string)
				sid, _ := reality["short_id"].(string)
				realityProto := map[string]any{
					"type":         "reality",
					"public_key":   pbk,
					"short_id":     sid,
					"sni_hostname": sni,
					"protocol":     vlessProto,
				}
				if strings.Contains(strings.ToLower(flow), "vision") {
					realityProto["vision"] = true
				}
				return map[string]any{
					"name":     tag,
					"address":  addr,
					"protocol": realityProto,
				}
			}

			tlsProto := map[string]any{
				"type":         "tls",
				"sni_hostname": sni,
				"verify":       !insecure,
				"protocol":     vlessProto,
			}
			return map[string]any{
				"name":     tag,
				"address":  addr,
				"protocol": tlsProto,
			}
		}

		return map[string]any{
			"name":     tag,
			"address":  addr,
			"protocol": vlessProto,
		}

	case "vmess":
		uuid, _ := ob["uuid"].(string)
		sec, _ := ob["security"].(string)
		// shoes supports: aes-128-gcm, chacha20-poly1305, none.
		// It rejects "auto" or "zero".
		if sec == "" || sec == "auto" || sec == "zero" {
			sec = "aes-128-gcm"
		}
		vmessProto := map[string]any{
			"type":    "vmess",
			"cipher":  sec,
			"user_id": uuid,
		}

		// check transport
		var innerProto any = vmessProto
		if transport, ok := ob["transport"].(map[string]any); ok {
			tType, _ := transport["type"].(string)
			if tType == "ws" {
				path, _ := transport["path"].(string)
				if path == "" {
					path = "/"
				}
				wsProto := map[string]any{
					"type":          "websocket",
					"matching_path": path,
					"protocol":      innerProto,
				}
				if headers, ok := transport["headers"].(map[string]any); ok {
					if host, ok := headers["Host"].(string); ok && host != "" {
						wsProto["matching_headers"] = map[string]string{"Host": host}
					}
				}
				innerProto = wsProto
			}
		}

		tlsMap, _ := ob["tls"].(map[string]any)
		if tlsMap != nil && boolVal(tlsMap["enabled"]) {
			sni, _ := tlsMap["server_name"].(string)
			if sni == "" {
				sni = server
			}
			insecure := boolVal(tlsMap["insecure"])
			innerProto = map[string]any{
				"type":         "tls",
				"sni_hostname": sni,
				"verify":       !insecure,
				"protocol":     innerProto,
			}
		}

		return map[string]any{
			"name":     tag,
			"address":  addr,
			"protocol": innerProto,
		}

	case "trojan":
		password, _ := ob["password"].(string)
		tlsMap, _ := ob["tls"].(map[string]any)
		sni := server
		insecure := false
		if tlsMap != nil {
			if s, _ := tlsMap["server_name"].(string); s != "" {
				sni = s
			}
			insecure = boolVal(tlsMap["insecure"])
		}
		return map[string]any{
			"name":    tag,
			"address": addr,
			"protocol": map[string]any{
				"type":         "tls",
				"sni_hostname": sni,
				"verify":       !insecure,
				"protocol": map[string]any{
					"type":     "trojan",
					"password": password,
				},
			},
		}

	case "socks":
		u, _ := ob["username"].(string)
		p, _ := ob["password"].(string)
		sProto := map[string]any{
			"type": "socks",
		}
		if u != "" {
			sProto["username"] = u
			sProto["password"] = p
		}
		return map[string]any{
			"name":     tag,
			"address":  addr,
			"protocol": sProto,
		}

	case "http":
		u, _ := ob["username"].(string)
		p, _ := ob["password"].(string)
		hProto := map[string]any{
			"type": "http",
		}
		if u != "" {
			hProto["username"] = u
			hProto["password"] = p
		}
		return map[string]any{
			"name":     tag,
			"address":  addr,
			"protocol": hProto,
		}

	default:
		return nil
	}
}

func anyIntVal(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	default:
		return 0
	}
}

func boolVal(v any) bool {
	if b, ok := v.(bool); ok {
		return b
	}
	return false
}
