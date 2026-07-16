package sharelink

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// parseClashLine handles classical Clash one-line proxies, e.g.:
//
//	Snell6 = snell, 216.236.7.38, 26005, psk = xxx, version = 6, reuse = true, tfo = true
//	HK = ss, 1.2.3.4, 8388, aes-256-gcm, password, udp=true
//
// Subscriptions and clipboard pastes often use this form instead of URI schemes.
func parseClashLine(line string) (Node, error) {
	line = strings.TrimSpace(line)
	eq := strings.Index(line, "=")
	if eq <= 0 {
		return Node{}, fmt.Errorf("not a clash proxy line")
	}
	name := strings.TrimSpace(line[:eq])
	rest := strings.TrimSpace(line[eq+1:])
	if name == "" || rest == "" {
		return Node{}, fmt.Errorf("not a clash proxy line")
	}

	parts := splitClashFields(rest)
	if len(parts) < 3 {
		return Node{}, fmt.Errorf("clash line: need type, server, port")
	}
	typ := strings.ToLower(strings.TrimSpace(parts[0]))
	server := strings.TrimSpace(parts[1])
	port := parsePort(strings.TrimSpace(parts[2]))
	if server == "" || port == 0 {
		return Node{}, fmt.Errorf("clash line: incomplete server/port")
	}

	kv := map[string]string{}
	var pos []string
	for _, p := range parts[3:] {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if k, v, ok := cutKV(p); ok {
			kv[normalizeClashKey(k)] = strings.TrimSpace(v)
			continue
		}
		pos = append(pos, p)
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
	case "ssr":
		return Node{}, fmt.Errorf("ssr is not supported by sing-box")
	default:
		return Node{}, fmt.Errorf("clash type %q not supported", typ)
	}
}

func looksLikeClashLine(line string) bool {
	eq := strings.Index(line, "=")
	if eq <= 0 {
		return false
	}
	// URI schemes already handled; avoid mistaking "key=value" alone
	rest := strings.TrimSpace(line[eq+1:])
	if rest == "" {
		return false
	}
	// type is first comma field
	typ := rest
	if i := strings.Index(rest, ","); i >= 0 {
		typ = rest[:i]
	}
	typ = strings.ToLower(strings.TrimSpace(typ))
	switch typ {
	case "snell", "ss", "shadowsocks", "ssr",
		"socks", "socks5", "socks5h", "socks4", "socks4a",
		"http", "https",
		"trojan", "vmess", "vless",
		"hysteria", "hy", "hysteria2", "hy2",
		"tuic", "anytls", "wireguard", "wg", "ssh":
		return true
	default:
		return false
	}
}

// splitClashFields splits on commas; values rarely contain commas in classic form.
func splitClashFields(s string) []string {
	return strings.Split(s, ",")
}

func cutKV(p string) (key, val string, ok bool) {
	// "psk = xxx" or "psk=xxx"
	i := strings.Index(p, "=")
	if i <= 0 {
		return "", "", false
	}
	return strings.TrimSpace(p[:i]), strings.TrimSpace(p[i+1:]), true
}

func normalizeClashKey(k string) string {
	k = strings.ToLower(strings.TrimSpace(k))
	k = strings.ReplaceAll(k, "_", "-")
	return k
}

func clashTruthy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func clashGet(kv map[string]string, keys ...string) string {
	for _, k := range keys {
		if v, ok := kv[normalizeClashKey(k)]; ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func applyClashDial(ob map[string]any, kv map[string]string) {
	if clashTruthy(clashGet(kv, "tfo", "tcp-fast-open", "tcp_fast_open")) {
		ob["tcp_fast_open"] = true
	}
	if clashTruthy(clashGet(kv, "udp")) {
		// sing-box uses network field; both enabled by default — no-op unless false
	}
	if v := clashGet(kv, "interface-name", "interface_name", "bind-interface"); v != "" {
		ob["bind_interface"] = v
	}
}

func clashSnell(tag, server string, port int, kv map[string]string) (Node, error) {
	psk := clashGet(kv, "psk", "password")
	if psk == "" {
		return Node{}, fmt.Errorf("snell: missing psk")
	}
	ver := 4
	if v := clashGet(kv, "version", "v"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			ver = n
		}
	}
	ob := map[string]any{
		"type":        "snell",
		"tag":         tag,
		"server":      server,
		"server_port": port,
		"version":     ver,
		"psk":         psk,
	}
	if uk := clashGet(kv, "userkey", "user-key"); uk != "" {
		ob["userkey"] = uk
	}
	if clashTruthy(clashGet(kv, "reuse")) {
		ob["reuse"] = true
	}
	if ver >= 6 {
		if mode := clashGet(kv, "mode"); mode != "" {
			ob["mode"] = mode
		}
	} else {
		if obfs := clashGet(kv, "obfs", "obfs-mode"); obfs != "" {
			ob["obfs_mode"] = obfs
		}
		if h := clashGet(kv, "obfs-host", "host"); h != "" {
			ob["obfs_host"] = h
		}
		// obfs-opts=mode=http;host=example.com
		if opts := clashGet(kv, "obfs-opts"); opts != "" {
			for _, seg := range strings.Split(opts, ";") {
				if k, v, ok := cutKV(seg); ok {
					switch normalizeClashKey(k) {
					case "mode":
						ob["obfs_mode"] = v
					case "host":
						ob["obfs_host"] = v
					}
				}
			}
		}
	}
	applyClashDial(ob, kv)
	return Node{Tag: tag, Outbound: ob}, nil
}

func clashSS(tag, server string, port int, pos []string, kv map[string]string) (Node, error) {
	// ss, server, port, method, password
	method := clashGet(kv, "cipher", "method")
	password := clashGet(kv, "password")
	if method == "" && len(pos) > 0 {
		method = pos[0]
	}
	if password == "" && len(pos) > 1 {
		password = pos[1]
	}
	if method == "" || password == "" {
		return Node{}, fmt.Errorf("ss: need method and password")
	}
	ob := map[string]any{
		"type":        "shadowsocks",
		"tag":         tag,
		"server":      server,
		"server_port": port,
		"method":      method,
		"password":    password,
	}
	if plugin := clashGet(kv, "plugin"); plugin != "" {
		ob["plugin"] = plugin
		if opts := clashGet(kv, "plugin-opts", "plugin_opts"); opts != "" {
			ob["plugin_opts"] = opts
		}
	}
	applyClashDial(ob, kv)
	return Node{Tag: tag, Outbound: ob}, nil
}

func clashSOCKS(tag, server string, port int, ver string, kv map[string]string) (Node, error) {
	ob := map[string]any{
		"type":        "socks",
		"tag":         tag,
		"server":      server,
		"server_port": port,
		"version":     ver,
	}
	if u := clashGet(kv, "username", "user"); u != "" {
		ob["username"] = u
	}
	if p := clashGet(kv, "password"); p != "" {
		ob["password"] = p
	}
	applyClashDial(ob, kv)
	return Node{Tag: tag, Outbound: ob}, nil
}

func clashHTTP(tag, server string, port int, tls bool, kv map[string]string) (Node, error) {
	ob := map[string]any{
		"type":        "http",
		"tag":         tag,
		"server":      server,
		"server_port": port,
	}
	if u := clashGet(kv, "username", "user"); u != "" {
		ob["username"] = u
	}
	if p := clashGet(kv, "password"); p != "" {
		ob["password"] = p
	}
	if tls || clashTruthy(clashGet(kv, "tls")) {
		sni := firstNonEmpty(clashGet(kv, "sni", "servername"), server)
		ob["tls"] = map[string]any{"enabled": true, "server_name": sni}
	}
	applyClashDial(ob, kv)
	return Node{Tag: tag, Outbound: ob}, nil
}

func clashTrojan(tag, server string, port int, pos []string, kv map[string]string) (Node, error) {
	password := clashGet(kv, "password")
	if password == "" && len(pos) > 0 {
		password = pos[0]
	}
	if password == "" {
		return Node{}, fmt.Errorf("trojan: missing password")
	}
	ob := map[string]any{
		"type":        "trojan",
		"tag":         tag,
		"server":      server,
		"server_port": port,
		"password":    password,
	}
	sni := firstNonEmpty(clashGet(kv, "sni", "servername", "peer"), server)
	insecure := clashTruthy(clashGet(kv, "skip-cert-verify", "insecure"))
	tls := map[string]any{"enabled": true, "server_name": sni}
	if insecure {
		tls["insecure"] = true
	}
	ob["tls"] = tls
	network := clashGet(kv, "network", "net")
	if network != "" && !strings.EqualFold(network, "tcp") {
		// minimal transport from network + opts
		q := url.Values{}
		if path := clashGet(kv, "ws-path", "path"); path != "" {
			q.Set("path", path)
		}
		if host := clashGet(kv, "ws-host", "host"); host != "" {
			q.Set("host", host)
		}
		if sn := clashGet(kv, "grpc-service-name", "service-name"); sn != "" {
			q.Set("serviceName", sn)
		}
		if t := buildV2RayTransport(network, q); t != nil {
			ob["transport"] = t
		}
	}
	applyClashDial(ob, kv)
	return Node{Tag: tag, Outbound: ob}, nil
}

func clashVMess(tag, server string, port int, pos []string, kv map[string]string) (Node, error) {
	uuid := clashGet(kv, "uuid", "id")
	if uuid == "" && len(pos) > 0 {
		uuid = pos[0]
	}
	if uuid == "" {
		return Node{}, fmt.Errorf("vmess: missing uuid")
	}
	var err error
	uuid, err = normalizeUUID(uuid)
	if err != nil {
		return Node{}, fmt.Errorf("vmess: %w", err)
	}
	security := clashGet(kv, "cipher", "security", "scy")
	if security == "" {
		security = "auto"
	}
	aid := 0
	if v := clashGet(kv, "alterid", "alter-id", "aid"); v != "" {
		aid, _ = strconv.Atoi(v)
	}
	ob := map[string]any{
		"type":        "vmess",
		"tag":         tag,
		"server":      server,
		"server_port": port,
		"uuid":        uuid,
		"security":    security,
		"alter_id":    aid,
	}
	network := clashGet(kv, "network", "net")
	if network == "" {
		network = "tcp"
	}
	q := url.Values{}
	if path := clashGet(kv, "ws-path", "path"); path != "" {
		q.Set("path", path)
	}
	if host := clashGet(kv, "ws-headers.host", "host", "ws-host"); host != "" {
		q.Set("host", host)
	}
	if sn := clashGet(kv, "grpc-service-name", "service-name"); sn != "" {
		q.Set("serviceName", sn)
	}
	if t := buildV2RayTransport(network, q); t != nil {
		ob["transport"] = t
	}
	if clashTruthy(clashGet(kv, "tls")) || clashGet(kv, "sni") != "" {
		sni := firstNonEmpty(clashGet(kv, "sni", "servername"), server)
		tls := map[string]any{"enabled": true, "server_name": sni}
		if clashTruthy(clashGet(kv, "skip-cert-verify", "insecure")) {
			tls["insecure"] = true
		}
		ob["tls"] = tls
	}
	applyClashDial(ob, kv)
	return Node{Tag: tag, Outbound: ob}, nil
}

func clashVLESS(tag, server string, port int, pos []string, kv map[string]string) (Node, error) {
	uuid := clashGet(kv, "uuid", "id")
	if uuid == "" && len(pos) > 0 {
		uuid = pos[0]
	}
	if uuid == "" {
		return Node{}, fmt.Errorf("vless: missing uuid")
	}
	var err error
	uuid, err = normalizeUUID(uuid)
	if err != nil {
		return Node{}, fmt.Errorf("vless: %w", err)
	}
	ob := map[string]any{
		"type":            "vless",
		"tag":             tag,
		"server":          server,
		"server_port":     port,
		"uuid":            uuid,
		"packet_encoding": "xudp",
	}
	if flow := clashGet(kv, "flow"); flow != "" && !strings.EqualFold(flow, "none") {
		ob["flow"] = flow
	}
	network := clashGet(kv, "network", "net", "type")
	if network == "" {
		network = "tcp"
	}
	q := url.Values{}
	if path := clashGet(kv, "ws-path", "path"); path != "" {
		q.Set("path", path)
	}
	if host := clashGet(kv, "host", "ws-host"); host != "" {
		q.Set("host", host)
	}
	if sn := clashGet(kv, "grpc-service-name", "service-name"); sn != "" {
		q.Set("serviceName", sn)
	}
	if t := buildV2RayTransport(network, q); t != nil {
		ob["transport"] = t
	}
	security := strings.ToLower(clashGet(kv, "security", "tls"))
	sni := firstNonEmpty(clashGet(kv, "sni", "servername"), server)
	fp := clashGet(kv, "client-fingerprint", "fp", "fingerprint")
	pbk := clashGet(kv, "public-key", "pbk", "reality-public-key")
	sid := clashGet(kv, "short-id", "sid")
	insecure := clashTruthy(clashGet(kv, "skip-cert-verify", "insecure"))
	if security == "true" || security == "1" {
		security = "tls"
	}
	if pbk != "" {
		security = "reality"
	}
	if security == "tls" || security == "reality" || clashTruthy(clashGet(kv, "tls")) {
		if tls := buildTLS(security, sni, fp, clashGet(kv, "alpn"), insecure, pbk, sid, ""); tls != nil {
			ob["tls"] = tls
		}
	}
	applyClashDial(ob, kv)
	return Node{Tag: tag, Outbound: ob}, nil
}

func clashHysteria(tag, server string, port int, kv map[string]string) (Node, error) {
	up := 100
	down := 100
	if v := clashGet(kv, "up", "up-mbps", "up_mbps"); v != "" {
		if n, err := strconv.Atoi(strings.TrimSpace(strings.Fields(v)[0])); err == nil {
			up = n
		}
	}
	if v := clashGet(kv, "down", "down-mbps", "down_mbps"); v != "" {
		if n, err := strconv.Atoi(strings.TrimSpace(strings.Fields(v)[0])); err == nil {
			down = n
		}
	}
	ob := map[string]any{
		"type":        "hysteria",
		"tag":         tag,
		"server":      server,
		"server_port": port,
		"up_mbps":     up,
		"down_mbps":   down,
	}
	if auth := clashGet(kv, "auth-str", "auth_str", "auth", "password"); auth != "" {
		ob["auth_str"] = auth
	}
	if obfs := clashGet(kv, "obfs"); obfs != "" {
		ob["obfs"] = obfs
	}
	sni := firstNonEmpty(clashGet(kv, "sni", "peer", "servername"), server)
	tls := map[string]any{"enabled": true, "server_name": sni}
	if clashTruthy(clashGet(kv, "skip-cert-verify", "insecure")) {
		tls["insecure"] = true
	}
	ob["tls"] = tls
	applyClashDial(ob, kv)
	return Node{Tag: tag, Outbound: ob}, nil
}

func clashHysteria2(tag, server string, port int, kv map[string]string) (Node, error) {
	password := clashGet(kv, "password", "auth")
	ob := map[string]any{
		"type":        "hysteria2",
		"tag":         tag,
		"server":      server,
		"server_port": port,
	}
	if password != "" {
		ob["password"] = password
	}
	if up := clashGet(kv, "up", "up-mbps"); up != "" {
		if n, err := strconv.Atoi(strings.TrimSpace(strings.Fields(up)[0])); err == nil {
			ob["up_mbps"] = n
		}
	}
	if down := clashGet(kv, "down", "down-mbps"); down != "" {
		if n, err := strconv.Atoi(strings.TrimSpace(strings.Fields(down)[0])); err == nil {
			ob["down_mbps"] = n
		}
	}
	obfsType := clashGet(kv, "obfs")
	obfsPass := clashGet(kv, "obfs-password", "obfs_password")
	if obfsType != "" && !strings.EqualFold(obfsType, "none") {
		o := map[string]any{"type": obfsType}
		if obfsPass != "" {
			o["password"] = obfsPass
		}
		ob["obfs"] = o
	} else if obfsPass != "" {
		ob["obfs"] = map[string]any{"type": "salamander", "password": obfsPass}
	}
	sni := firstNonEmpty(clashGet(kv, "sni", "servername"), server)
	tls := map[string]any{"enabled": true, "server_name": sni}
	if clashTruthy(clashGet(kv, "skip-cert-verify", "insecure")) {
		tls["insecure"] = true
	}
	ob["tls"] = tls
	applyClashDial(ob, kv)
	return Node{Tag: tag, Outbound: ob}, nil
}

func clashTUIC(tag, server string, port int, kv map[string]string) (Node, error) {
	uuid := clashGet(kv, "uuid")
	password := clashGet(kv, "password")
	if uuid == "" {
		return Node{}, fmt.Errorf("tuic: missing uuid")
	}
	if u2, p2 := splitUUIDPassword(uuid); p2 != "" {
		uuid = u2
		if password == "" {
			password = p2
		}
	}
	var err error
	uuid, err = normalizeUUID(uuid)
	if err != nil {
		return Node{}, fmt.Errorf("tuic: %w", err)
	}
	ob := map[string]any{
		"type":        "tuic",
		"tag":         tag,
		"server":      server,
		"server_port": port,
		"uuid":        uuid,
	}
	if password != "" {
		ob["password"] = password
	}
	if cc := clashGet(kv, "congestion-controller", "congestion-control"); cc != "" {
		ob["congestion_control"] = cc
	}
	if urm := clashGet(kv, "udp-relay-mode"); urm != "" {
		ob["udp_relay_mode"] = urm
	}
	sni := firstNonEmpty(clashGet(kv, "sni", "servername"), server)
	tls := map[string]any{"enabled": true, "server_name": sni, "alpn": []any{"h3"}}
	if clashTruthy(clashGet(kv, "skip-cert-verify", "insecure")) {
		tls["insecure"] = true
	}
	ob["tls"] = tls
	applyClashDial(ob, kv)
	return Node{Tag: tag, Outbound: ob}, nil
}

func clashAnyTLS(tag, server string, port int, kv map[string]string) (Node, error) {
	password := clashGet(kv, "password")
	if password == "" {
		return Node{}, fmt.Errorf("anytls: missing password")
	}
	ob := map[string]any{
		"type":        "anytls",
		"tag":         tag,
		"server":      server,
		"server_port": port,
		"password":    password,
	}
	sni := firstNonEmpty(clashGet(kv, "sni", "servername"), server)
	tls := map[string]any{"enabled": true, "server_name": sni}
	if clashTruthy(clashGet(kv, "skip-cert-verify", "insecure")) {
		tls["insecure"] = true
	}
	ob["tls"] = tls
	applyClashDial(ob, kv)
	return Node{Tag: tag, Outbound: ob}, nil
}

func clashWireGuard(tag, server string, port int, kv map[string]string) (Node, error) {
	priv := clashGet(kv, "private-key", "private_key", "privatekey")
	pub := clashGet(kv, "public-key", "public_key", "publickey", "peer-public-key")
	if priv == "" || pub == "" {
		return Node{}, fmt.Errorf("wireguard: need private-key and public-key")
	}
	addr := clashGet(kv, "ip", "address", "local-address")
	if addr == "" {
		addr = "10.0.0.2/32"
	}
	var local []any
	for _, a := range strings.Split(addr, ",") {
		a = strings.TrimSpace(a)
		if a != "" {
			local = append(local, a)
		}
	}
	ob := map[string]any{
		"type":            "wireguard",
		"tag":             tag,
		"server":          server,
		"server_port":     port,
		"private_key":     priv,
		"peer_public_key": pub,
		"local_address":   local,
	}
	if psk := clashGet(kv, "preshared-key", "pre-shared-key", "psk"); psk != "" {
		ob["pre_shared_key"] = psk
	}
	if mtu := clashGet(kv, "mtu"); mtu != "" {
		if n, err := strconv.Atoi(mtu); err == nil {
			ob["mtu"] = n
		}
	}
	applyClashDial(ob, kv)
	return Node{Tag: tag, Outbound: ob}, nil
}

func clashSSH(tag, server string, port int, kv map[string]string) (Node, error) {
	user := clashGet(kv, "username", "user")
	if user == "" {
		user = "root"
	}
	ob := map[string]any{
		"type":        "ssh",
		"tag":         tag,
		"server":      server,
		"server_port": port,
		"user":        user,
	}
	if p := clashGet(kv, "password"); p != "" {
		ob["password"] = p
	}
	if pk := clashGet(kv, "private-key", "private_key"); pk != "" {
		ob["private_key"] = pk
	}
	applyClashDial(ob, kv)
	return Node{Tag: tag, Outbound: ob}, nil
}
