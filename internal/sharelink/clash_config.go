package sharelink

import (
	"encoding/json"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// ParseClashConfig extracts proxies from a Clash / Clash Meta config body
// (YAML or JSON). Subscription panels often return this for multi-protocol
// lists (ss + snell + vless + …) instead of base64 URI lines.
func ParseClashConfig(body string) ([]Node, error) {
	body = strings.TrimSpace(body)
	if body == "" {
		return nil, fmt.Errorf("empty clash config")
	}
	// Quick reject: pure URI subscription, not Clash.
	if !looksLikeClashConfig(body) {
		return nil, fmt.Errorf("not a clash config")
	}

	var root map[string]any
	// Prefer YAML (Clash default); JSON is a subset YAML parsers often accept,
	// but use encoding/json when body is clearly JSON for better error messages.
	if strings.HasPrefix(body, "{") || strings.HasPrefix(body, "[") {
		if err := json.Unmarshal([]byte(body), &root); err != nil {
			// Some panels send YAML that starts with --- or comments only;
			// fall through to YAML.
			if err := yaml.Unmarshal([]byte(body), &root); err != nil {
				return nil, fmt.Errorf("clash json/yaml: %w", err)
			}
		}
	} else {
		if err := yaml.Unmarshal([]byte(body), &root); err != nil {
			return nil, fmt.Errorf("clash yaml: %w", err)
		}
	}

	rawProxies, ok := root["proxies"]
	if !ok {
		return nil, fmt.Errorf("clash config: no proxies")
	}
	list, ok := rawProxies.([]any)
	if !ok {
		return nil, fmt.Errorf("clash config: proxies is not a list")
	}

	var nodes []Node
	var errs []string
	for _, item := range list {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		n, err := nodeFromClashProxyMap(m)
		if err != nil {
			errs = append(errs, err.Error())
			continue
		}
		nodes = append(nodes, n)
	}
	if len(nodes) == 0 {
		if len(errs) > 0 {
			return nil, fmt.Errorf("clash proxies: %s", errs[0])
		}
		return nil, fmt.Errorf("clash proxies: none parsed")
	}
	return nodes, nil
}

func looksLikeClashConfig(body string) bool {
	// Strip UTF-8 BOM
	body = strings.TrimPrefix(body, "\ufeff")
	trim := strings.TrimSpace(body)
	low := strings.ToLower(trim)
	if strings.Contains(low, "proxies:") || strings.Contains(low, `"proxies"`) || strings.Contains(low, "'proxies'") {
		return true
	}
	// JSON object with proxies key (compact)
	if strings.HasPrefix(trim, "{") && strings.Contains(low, "proxies") {
		return true
	}
	return false
}

// nodeFromClashProxyMap converts one Clash proxy object into a sing-box outbound.
func nodeFromClashProxyMap(m map[string]any) (Node, error) {
	typ := strings.ToLower(strings.TrimSpace(anyString(m["type"])))
	if typ == "" {
		return Node{}, fmt.Errorf("proxy missing type")
	}
	name := strings.TrimSpace(anyString(m["name"]))
	if name == "" {
		name = typ + "-node"
	}
	server := strings.TrimSpace(anyString(m["server"]))
	port := anyInt(m["port"])
	if server == "" || port == 0 {
		return Node{}, fmt.Errorf("%s %q: incomplete server/port", typ, name)
	}

	// Flatten scalar fields (+ one level of nested maps like obfs-opts / plugin-opts)
	// into the same key space as classical Clash lines.
	kv := map[string]string{}
	var pos []string
	for k, v := range m {
		lk := normalizeClashKey(k)
		switch lk {
		case "name", "type", "server", "port":
			continue
		}
		switch val := v.(type) {
		case map[string]any: // same as map[string]interface{} in Go
			for nk, nv := range val {
				// obfs-opts.mode → mode (and keep nested key)
				kv[normalizeClashKey(nk)] = fmt.Sprint(nv)
				kv[normalizeClashKey(k)+"-"+normalizeClashKey(nk)] = fmt.Sprint(nv)
			}
		case []any, []string:
			// e.g. alpn: [h3, h2] — join
			kv[lk] = joinAnySlice(v)
		case nil:
			continue
		default:
			kv[lk] = strings.TrimSpace(fmt.Sprint(v))
		}
	}

	tag := sanitizeTag(name)
	switch typ {
	case "snell":
		return clashSnell(tag, server, port, kv)
	case "ss", "shadowsocks":
		return clashSS(tag, server, port, pos, kv)
	case "socks", "socks5", "socks5h":
		return clashSOCKS(tag, server, port, "5", kv)
	case "socks4", "socks4a":
		ver := "4"
		if typ == "socks4a" {
			ver = "4a"
		}
		return clashSOCKS(tag, server, port, ver, kv)
	case "http", "https":
		return clashHTTP(tag, server, port, typ == "https", kv)
	case "trojan":
		return clashTrojan(tag, server, port, pos, kv)
	case "vmess":
		return clashVMess(tag, server, port, pos, kv)
	case "vless":
		return clashVLESS(tag, server, port, pos, kv)
	case "hysteria", "hy":
		return clashHysteria(tag, server, port, kv)
	case "hysteria2", "hy2":
		return clashHysteria2(tag, server, port, kv)
	case "tuic":
		return clashTUIC(tag, server, port, kv)
	case "anytls":
		return clashAnyTLS(tag, server, port, kv)
	case "wireguard", "wg":
		return clashWireGuard(tag, server, port, kv)
	case "ssh":
		return clashSSH(tag, server, port, kv)
	case "naive", "naiveproxy":
		return clashNaive(tag, server, port, kv)
	case "ssr":
		return Node{}, fmt.Errorf("ssr is not supported by sing-box")
	default:
		return Node{}, fmt.Errorf("clash type %q not supported", typ)
	}
}

func joinAnySlice(v any) string {
	switch s := v.(type) {
	case []string:
		return strings.Join(s, ",")
	case []any:
		parts := make([]string, 0, len(s))
		for _, x := range s {
			parts = append(parts, fmt.Sprint(x))
		}
		return strings.Join(parts, ",")
	default:
		return fmt.Sprint(v)
	}
}

// clashNaive maps Clash-style naive proxy entries (if a panel exports them).
func clashNaive(tag, server string, port int, kv map[string]string) (Node, error) {
	user := clashGet(kv, "username", "user")
	pass := clashGet(kv, "password", "pass")
	if user == "" && pass == "" {
		return Node{}, fmt.Errorf("naive: missing credentials")
	}
	ob := map[string]any{
		"type":        "naive",
		"tag":         tag,
		"server":      server,
		"server_port": port,
		"username":    user,
		"password":    pass,
	}
	// Clash often uses "tls: true" for HTTPS naive
	if clashTruthy(clashGet(kv, "tls", "https")) || strings.EqualFold(clashGet(kv, "network", "protocol"), "https") {
		sni := firstNonEmpty(clashGet(kv, "sni", "servername", "host"), server)
		ob["tls"] = map[string]any{"enabled": true, "server_name": sni}
	}
	applyClashDial(ob, kv)
	return Node{Tag: tag, Outbound: ob}, nil
}
