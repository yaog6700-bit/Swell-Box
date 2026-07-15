package sharelink

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// Node is a parsed share link ready to become a sing-box outbound.
type Node struct {
	Tag      string
	Outbound map[string]any
}

// SupportedSchemes lists share-link schemes handled by Parse (for docs/errors).
var SupportedSchemes = []string{
	"ss", "vmess", "vless", "trojan",
	"hysteria", "hy", "hysteria2", "hy2",
	"tuic", "anytls",
	"wireguard", "wg",
	"socks", "socks5", "socks5h", "socks4", "socks4a",
	"http", "https",
	"snell",
	"naive+https", "naive+http", "naive+quic",
	"ssh",
}

// Parse detects supported share links (and multi-line paste).
// Subscription bodies that are base64-encoded multi-line lists are handled by the subscribe package.
func Parse(raw string) ([]Node, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("clipboard is empty")
	}
	var lines []string
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// allow blank lines / comments in pasted lists
		if strings.HasPrefix(line, "#") && !strings.Contains(line, "://") {
			continue
		}
		lines = append(lines, line)
	}
	if len(lines) == 0 {
		return nil, fmt.Errorf("no link found")
	}

	var nodes []Node
	var errs []string
	for _, line := range lines {
		n, err := parseOne(line)
		if err != nil {
			errs = append(errs, err.Error())
			continue
		}
		nodes = append(nodes, n)
	}
	if len(nodes) == 0 {
		if len(errs) > 0 {
			return nil, fmt.Errorf("%s", errs[0])
		}
		return nil, fmt.Errorf("unsupported link (need %s)", schemesHint())
	}
	return nodes, nil
}

func schemesHint() string {
	return "ss/vmess/vless/trojan/hysteria(2)/tuic/anytls/wireguard/socks/http/snell/naive/ssh"
}

func parseOne(line string) (Node, error) {
	// strip surrounding quotes sometimes pasted from editors
	line = strings.Trim(line, " \t\"'")
	lower := strings.ToLower(line)

	// scheme detection (order: multi-word schemes first)
	switch {
	case strings.HasPrefix(lower, "ss://"):
		return parseSS(line)
	case strings.HasPrefix(lower, "ssr://"):
		return Node{}, fmt.Errorf("ssr:// is not supported by sing-box (use ss/vmess/vless/…)")
	case strings.HasPrefix(lower, "vmess://"):
		return parseVMess(line)
	case strings.HasPrefix(lower, "vless://"):
		return parseVLESS(line)
	case strings.HasPrefix(lower, "trojan://"):
		return parseTrojan(line)
	case strings.HasPrefix(lower, "hysteria2://"), strings.HasPrefix(lower, "hy2://"):
		return parseHysteria2(line)
	case strings.HasPrefix(lower, "hysteria://"), strings.HasPrefix(lower, "hy://"):
		return parseHysteria(line)
	case strings.HasPrefix(lower, "tuic://"):
		return parseTUIC(line)
	case strings.HasPrefix(lower, "anytls://"):
		return parseAnyTLS(line)
	case strings.HasPrefix(lower, "wireguard://"), strings.HasPrefix(lower, "wg://"):
		return parseWireGuard(line)
	case strings.HasPrefix(lower, "socks5h://"), strings.HasPrefix(lower, "socks5://"),
		strings.HasPrefix(lower, "socks4a://"), strings.HasPrefix(lower, "socks4://"),
		strings.HasPrefix(lower, "socks://"):
		return parseSOCKS(line)
	case strings.HasPrefix(lower, "snell://"):
		return parseSnell(line)
	case strings.HasPrefix(lower, "naive+https://"), strings.HasPrefix(lower, "naive+http://"),
		strings.HasPrefix(lower, "naive+quic://"), strings.HasPrefix(lower, "naive://"):
		return parseNaive(line)
	case strings.HasPrefix(lower, "ssh://"):
		return parseSSH(line)
	case strings.HasPrefix(lower, "https://"):
		// NaiveProxy is commonly shared as:
		//   https://user:pass@host:port#name
		// Prefer naive when credentials are present; bare host:port stays HTTP proxy.
		if looksLikeNaiveHTTPS(line) {
			return parseNaive(line)
		}
		return parseHTTPProxy(line)
	case strings.HasPrefix(lower, "http://"):
		return parseHTTPProxy(line)
	default:
		// Clash classical one-line: Name = type, server, port, key = value, ...
		if looksLikeClashLine(line) {
			return parseClashLine(line)
		}
		return Node{}, fmt.Errorf("unsupported scheme (need %s or Clash line)", schemesHint())
	}
}

// ─── Shadowsocks ─────────────────────────────────────────────────────────────

func parseSS(line string) (Node, error) {
	// ss://base64(method:password)@host:port#name
	// or ss://base64(method:password@host:port)#name
	// SIP002: ss://base64(method:password)@host:port/?plugin=...#name
	u, err := url.Parse(line)
	if err != nil {
		return Node{}, fmt.Errorf("bad ss link: %w", err)
	}

	var method, password, host string
	var port int

	if u.User != nil && u.Host != "" {
		userInfo := u.User.String()
		if p, ok := u.User.Password(); ok {
			method = u.User.Username()
			password = p
		} else {
			decoded, err := decodeBase64(userInfo)
			if err != nil {
				return Node{}, fmt.Errorf("ss userinfo decode: %w", err)
			}
			method, password, err = splitMethodPass(decoded)
			if err != nil {
				return Node{}, err
			}
		}
		host = u.Hostname()
		port, _ = strconv.Atoi(u.Port())
	} else {
		body := strings.TrimPrefix(line, "ss://")
		body = strings.TrimPrefix(body, "SS://")
		if i := strings.Index(body, "#"); i >= 0 {
			body = body[:i]
		}
		if i := strings.Index(body, "?"); i >= 0 {
			body = body[:i]
		}
		// sometimes still method:password@host:port after base64
		decoded, err := decodeBase64(body)
		if err != nil {
			return Node{}, fmt.Errorf("ss body decode: %w", err)
		}
		at := strings.LastIndex(decoded, "@")
		if at < 0 {
			return Node{}, fmt.Errorf("ss: missing @")
		}
		method, password, err = splitMethodPass(decoded[:at])
		if err != nil {
			return Node{}, err
		}
		host, port, err = splitHostPort(decoded[at+1:])
		if err != nil {
			return Node{}, err
		}
	}
	if host == "" || port == 0 || method == "" || password == "" {
		return Node{}, fmt.Errorf("ss: incomplete fields")
	}

	tag := tagFromFragment(u.Fragment, "ss-"+host)
	ob := map[string]any{
		"type":        "shadowsocks",
		"tag":         tag,
		"server":      host,
		"server_port": port,
		"method":      method,
		"password":    password,
	}

	// SIP003 plugin
	if plugin := u.Query().Get("plugin"); plugin != "" {
		// plugin=v2ray-plugin;mode=websocket;host=example.com
		name, opts, _ := strings.Cut(plugin, ";")
		name = strings.TrimSpace(name)
		// normalize common aliases
		switch strings.ToLower(name) {
		case "simple-obfs", "obfs":
			name = "obfs-local"
		}
		if name != "" {
			ob["plugin"] = name
			if opts != "" {
				ob["plugin_opts"] = opts
			}
		}
	}

	return Node{Tag: tag, Outbound: ob}, nil
}

// ─── VMess ───────────────────────────────────────────────────────────────────

func parseVMess(line string) (Node, error) {
	body := line
	if i := strings.Index(strings.ToLower(body), "vmess://"); i >= 0 {
		body = body[i+len("vmess://"):]
	}
	// fragment not part of base64 body usually
	if i := strings.Index(body, "#"); i >= 0 {
		// some clients append #name after base64; keep for tag fallback
		// but base64 must not include fragment
		// we'll re-parse fragment via url if needed
	}

	// Try standard v2rayN base64 JSON first
	rawBody := body
	frag := ""
	if i := strings.Index(rawBody, "#"); i >= 0 {
		frag = rawBody[i+1:]
		rawBody = rawBody[:i]
	}
	decoded, err := decodeBase64(rawBody)
	if err == nil && strings.HasPrefix(strings.TrimSpace(decoded), "{") {
		return parseVMessJSON(decoded, frag)
	}

	// Fallback: URI style vmess://uuid@host:port?...
	u, err := url.Parse(line)
	if err != nil {
		return Node{}, fmt.Errorf("bad vmess link: %w", err)
	}
	uuid := ""
	if u.User != nil {
		uuid = u.User.Username()
	}
	host := u.Hostname()
	port, _ := strconv.Atoi(u.Port())
	if uuid == "" || host == "" || port == 0 {
		return Node{}, fmt.Errorf("vmess: incomplete fields (need base64 JSON or uuid@host:port)")
	}
	q := u.Query()
	tag := tagFromFragment(u.Fragment, "vmess-"+host)
	// encryption / scy = cipher; security query often means TLS (handled by applyTLS)
	enc := queryFirst(q, "encryption", "scy")
	if enc == "" {
		enc = "auto"
	}
	ob := map[string]any{
		"type":        "vmess",
		"tag":         tag,
		"server":      host,
		"server_port": port,
		"uuid":        uuid,
		"security":    enc,
		"alter_id":    queryInt(q, "aid", "alterId", "alter_id"),
	}
	network := queryFirst(q, "type", "net", "network")
	if network == "" {
		network = "tcp"
	}
	applyTransport(ob, network, q)
	applyTLS(ob, q, "")
	applyPacketEncoding(ob, q)
	applyMultiplex(ob, q)
	return Node{Tag: tag, Outbound: ob}, nil
}

func parseVMessJSON(decoded, frag string) (Node, error) {
	var m map[string]any
	if err := json.Unmarshal([]byte(decoded), &m); err != nil {
		return Node{}, fmt.Errorf("vmess json: %w", err)
	}
	host := anyString(m["add"])
	port := anyInt(m["port"])
	uuid := anyString(m["id"])
	if host == "" || port == 0 || uuid == "" {
		return Node{}, fmt.Errorf("vmess: incomplete fields")
	}
	ps := anyString(m["ps"])
	if ps == "" {
		ps = tagFromFragment(frag, "vmess-"+host)
	} else {
		ps = sanitizeTag(ps)
	}
	security := anyString(m["scy"])
	if security == "" {
		security = anyString(m["security"])
	}
	if security == "" || security == "tls" || security == "reality" {
		// scy is cipher; tls goes elsewhere
		if security == "tls" || security == "reality" {
			security = "auto"
		} else if security == "" {
			security = "auto"
		}
	}
	ob := map[string]any{
		"type":        "vmess",
		"tag":         ps,
		"server":      host,
		"server_port": port,
		"uuid":        uuid,
		"security":    security,
		"alter_id":    anyInt(m["aid"]),
	}
	network := anyString(m["net"])
	if network == "" {
		network = "tcp"
	}
	// Build synthetic query for transport/tls helpers
	q := url.Values{}
	if path := anyString(m["path"]); path != "" {
		q.Set("path", path)
	}
	if h := anyString(m["host"]); h != "" {
		q.Set("host", h)
	}
	if t := anyString(m["type"]); t != "" {
		q.Set("headerType", t)
	}
	if sn := anyString(m["serviceName"]); sn != "" {
		q.Set("serviceName", sn)
	} else if sn := anyString(m["service_name"]); sn != "" {
		q.Set("serviceName", sn)
	}
	if mode := anyString(m["mode"]); mode != "" {
		q.Set("mode", mode)
	}
	applyTransport(ob, network, q)

	// TLS / Reality from JSON
	tlsFlag := strings.ToLower(anyString(m["tls"]))
	sni := firstNonEmpty(anyString(m["sni"]), anyString(m["peer"]))
	if sni == "" && anyString(m["host"]) != "" && (tlsFlag == "tls" || tlsFlag == "reality") {
		// often host is SNI for ws+tls
		sni = anyString(m["host"])
	}
	fp := anyString(m["fp"])
	alpn := anyString(m["alpn"])
	pbk := firstNonEmpty(anyString(m["pbk"]), anyString(m["publicKey"]))
	sid := firstNonEmpty(anyString(m["sid"]), anyString(m["shortId"]))
	insecure := anyBool(m["allowInsecure"]) || anyBool(m["insecure"])
	securityTLS := tlsFlag
	if securityTLS == "" && pbk != "" {
		securityTLS = "reality"
	}
	if tls := buildTLS(securityTLS, sni, fp, alpn, insecure, pbk, sid, anyString(m["spx"])); tls != nil {
		ob["tls"] = tls
	}
	if pe := anyString(m["packetEncoding"]); pe != "" {
		ob["packet_encoding"] = pe
	}
	return Node{Tag: ps, Outbound: ob}, nil
}

// ─── VLESS ───────────────────────────────────────────────────────────────────

func parseVLESS(line string) (Node, error) {
	u, err := url.Parse(line)
	if err != nil {
		return Node{}, fmt.Errorf("bad vless link: %w", err)
	}
	uuid := ""
	if u.User != nil {
		uuid = u.User.Username()
	}
	host := u.Hostname()
	port, _ := strconv.Atoi(u.Port())
	if uuid == "" || host == "" || port == 0 {
		return Node{}, fmt.Errorf("vless: incomplete fields")
	}
	q := u.Query()
	tag := tagFromFragment(u.Fragment, "vless-"+host)
	ob := map[string]any{
		"type":            "vless",
		"tag":             tag,
		"server":          host,
		"server_port":     port,
		"uuid":            uuid,
		"packet_encoding": "xudp",
	}
	applyFlow(ob, q)
	// encryption is usually none; not a sing-box field for vless
	network := queryFirst(q, "type", "network", "net")
	if network == "" {
		network = "tcp"
	}
	// For reality/ws, host query is often WS Host header; SNI is sni=
	// Avoid treating host as SNI when type=ws — buildTLSFromQuery uses host as sni fallback.
	// Prefer only sni/peer for TLS SNI:
	qTLS := cloneValues(q)
	if strings.EqualFold(network, "ws") || strings.EqualFold(network, "websocket") ||
		strings.EqualFold(network, "httpupgrade") || strings.EqualFold(network, "grpc") {
		// remove host from SNI candidate by clearing if sni present or using peer only
		if qTLS.Get("sni") == "" && qTLS.Get("peer") == "" && qTLS.Get("servername") == "" {
			// leave host as last-resort SNI (common for grpc/ws without sni)
		}
	}
	applyTLS(ob, qTLS, "")
	applyTransport(ob, network, q)
	applyPacketEncoding(ob, q)
	applyMultiplex(ob, q)
	return Node{Tag: tag, Outbound: ob}, nil
}

// ─── Trojan ──────────────────────────────────────────────────────────────────

func parseTrojan(line string) (Node, error) {
	u, err := url.Parse(line)
	if err != nil {
		return Node{}, fmt.Errorf("bad trojan link: %w", err)
	}
	password := ""
	if u.User != nil {
		// password may contain special chars; User.Username() is the password for trojan
		password = u.User.Username()
		if p, ok := u.User.Password(); ok && p != "" {
			// rare user:pass form — use full userinfo as password if password set
			password = u.User.String()
			if password == "" {
				password = p
			}
		}
		// URL-decode
		if d, err := url.QueryUnescape(password); err == nil {
			password = d
		}
	}
	host := u.Hostname()
	port, _ := strconv.Atoi(u.Port())
	if password == "" || host == "" || port == 0 {
		return Node{}, fmt.Errorf("trojan: incomplete fields")
	}
	q := u.Query()
	tag := tagFromFragment(u.Fragment, "trojan-"+host)
	ob := map[string]any{
		"type":        "trojan",
		"tag":         tag,
		"server":      host,
		"server_port": port,
		"password":    password,
	}
	network := queryFirst(q, "type", "network", "net")
	if network == "" {
		network = "tcp"
	}
	// trojan almost always TLS; default security=tls if unset
	applyTLS(ob, q, "tls")
	if _, ok := ob["tls"]; !ok {
		// force TLS with host as SNI
		ob["tls"] = map[string]any{
			"enabled":     true,
			"server_name": firstNonEmpty(queryFirst(q, "sni", "peer"), host),
		}
	}
	applyTransport(ob, network, q)
	applyMultiplex(ob, q)
	return Node{Tag: tag, Outbound: ob}, nil
}

// ─── Hysteria 1 ──────────────────────────────────────────────────────────────

func parseHysteria(line string) (Node, error) {
	// hysteria://host:port?auth=...&peer=...&upmbps=&downmbps=&obfs=&alpn=
	// hysteria://auth@host:port?...
	u, err := url.Parse(line)
	if err != nil {
		return Node{}, fmt.Errorf("bad hysteria link: %w", err)
	}
	host := u.Hostname()
	port, _ := strconv.Atoi(u.Port())
	if host == "" || port == 0 {
		return Node{}, fmt.Errorf("hysteria: incomplete fields")
	}
	q := u.Query()
	auth := queryFirst(q, "auth", "auth_str", "auth-str", "password")
	if auth == "" && u.User != nil {
		auth = u.User.Username()
		if p, ok := u.User.Password(); ok && p != "" {
			auth = u.User.String()
		}
		if d, err := url.QueryUnescape(auth); err == nil {
			auth = d
		}
	}
	tag := tagFromFragment(u.Fragment, "hy-"+host)
	up := queryInt(q, "upmbps", "up_mbps", "up")
	down := queryInt(q, "downmbps", "down_mbps", "down")
	if up <= 0 {
		up = 100
	}
	if down <= 0 {
		down = 100
	}
	ob := map[string]any{
		"type":        "hysteria",
		"tag":         tag,
		"server":      host,
		"server_port": port,
		"up_mbps":     up,
		"down_mbps":   down,
	}
	if auth != "" {
		ob["auth_str"] = auth
	}
	if obfs := queryFirst(q, "obfs", "obfsParam", "obfs-password"); obfs != "" {
		ob["obfs"] = obfs
	}
	// TLS required
	applyTLS(ob, q, "tls")
	if _, ok := ob["tls"]; !ok {
		sni := firstNonEmpty(queryFirst(q, "peer", "sni"), host)
		tls := map[string]any{"enabled": true, "server_name": sni}
		if queryBool(q, "insecure", "allowInsecure") {
			tls["insecure"] = true
		}
		if alpn := parseALPN(queryFirst(q, "alpn")); len(alpn) > 0 {
			tls["alpn"] = alpn
		}
		ob["tls"] = tls
	}
	return Node{Tag: tag, Outbound: ob}, nil
}

// ─── Hysteria2 ───────────────────────────────────────────────────────────────

func parseHysteria2(line string) (Node, error) {
	// hysteria2://password@host:port?sni=&obfs=salamander&obfs-password=
	u, err := url.Parse(line)
	if err != nil {
		return Node{}, fmt.Errorf("bad hysteria2 link: %w", err)
	}
	password := ""
	if u.User != nil {
		password = u.User.Username()
		if p, ok := u.User.Password(); ok && p != "" {
			// user:pass form → password is "user:pass" for official hysteria2 userpass
			password = u.User.Username() + ":" + p
		}
		if d, err := url.QueryUnescape(password); err == nil {
			password = d
		}
	}
	host := u.Hostname()
	port, _ := strconv.Atoi(u.Port())
	q := u.Query()
	// password also in query for some links
	if password == "" {
		password = queryFirst(q, "auth", "password")
	}
	if host == "" || (port == 0 && queryFirst(q, "mport", "server_ports") == "") {
		return Node{}, fmt.Errorf("hysteria2: incomplete fields")
	}
	if port == 0 {
		port = 443
	}
	tag := tagFromFragment(u.Fragment, "hy2-"+host)
	ob := map[string]any{
		"type":        "hysteria2",
		"tag":         tag,
		"server":      host,
		"server_port": port,
	}
	if password != "" {
		ob["password"] = password
	}
	if up := queryInt(q, "upmbps", "up_mbps", "up"); up > 0 {
		ob["up_mbps"] = up
	}
	if down := queryInt(q, "downmbps", "down_mbps", "down"); down > 0 {
		ob["down_mbps"] = down
	}
	// port hopping
	if mport := queryFirst(q, "mport", "server_ports", "ports"); mport != "" {
		// formats: "2000-3000" or "2000:3000" or "2000,2001"
		mport = strings.ReplaceAll(mport, "-", ":")
		var ports []any
		for _, p := range strings.Split(mport, ",") {
			p = strings.TrimSpace(p)
			if p != "" {
				ports = append(ports, p)
			}
		}
		if len(ports) > 0 {
			ob["server_ports"] = ports
			delete(ob, "server_port")
		}
	}
	// obfs
	obfsType := strings.ToLower(queryFirst(q, "obfs"))
	obfsPass := queryFirst(q, "obfs-password", "obfs_password", "obfsPassword", "obfsparam", "obfsParam")
	if obfsType != "" && obfsType != "none" {
		obfs := map[string]any{"type": obfsType}
		if obfsPass != "" {
			obfs["password"] = obfsPass
		}
		ob["obfs"] = obfs
	} else if obfsPass != "" {
		ob["obfs"] = map[string]any{"type": "salamander", "password": obfsPass}
	}
	applyTLS(ob, q, "tls")
	if _, ok := ob["tls"]; !ok {
		sni := firstNonEmpty(queryFirst(q, "sni", "peer"), host)
		tls := map[string]any{"enabled": true, "server_name": sni}
		if queryBool(q, "insecure", "allowInsecure") {
			tls["insecure"] = true
		}
		if alpn := parseALPN(queryFirst(q, "alpn")); len(alpn) > 0 {
			tls["alpn"] = alpn
		}
		ob["tls"] = tls
	}
	return Node{Tag: tag, Outbound: ob}, nil
}

// ─── TUIC ────────────────────────────────────────────────────────────────────

func parseTUIC(line string) (Node, error) {
	// tuic://uuid:password@host:port?congestion_control=&udp_relay_mode=&sni=&alpn=
	u, err := url.Parse(line)
	if err != nil {
		return Node{}, fmt.Errorf("bad tuic link: %w", err)
	}
	uuid, password := "", ""
	if u.User != nil {
		uuid = u.User.Username()
		password, _ = u.User.Password()
		// sometimes uuid only, password in query
	}
	q := u.Query()
	if password == "" {
		password = queryFirst(q, "password", "pass")
	}
	if uuid == "" {
		uuid = queryFirst(q, "uuid")
	}
	host := u.Hostname()
	port, _ := strconv.Atoi(u.Port())
	if uuid == "" || host == "" || port == 0 {
		return Node{}, fmt.Errorf("tuic: incomplete fields")
	}
	tag := tagFromFragment(u.Fragment, "tuic-"+host)
	ob := map[string]any{
		"type":        "tuic",
		"tag":         tag,
		"server":      host,
		"server_port": port,
		"uuid":        uuid,
	}
	if password != "" {
		ob["password"] = password
	}
	if cc := queryFirst(q, "congestion_control", "congestion-control", "congestion"); cc != "" {
		ob["congestion_control"] = cc
	}
	if urm := queryFirst(q, "udp_relay_mode", "udp-relay-mode"); urm != "" {
		ob["udp_relay_mode"] = urm
	}
	if queryBool(q, "udp_over_stream", "udp-over-stream") {
		ob["udp_over_stream"] = true
	}
	if queryBool(q, "zero_rtt_handshake", "zero-rtt-handshake") {
		ob["zero_rtt_handshake"] = true
	}
	applyTLS(ob, q, "tls")
	if _, ok := ob["tls"]; !ok {
		sni := firstNonEmpty(queryFirst(q, "sni", "peer"), host)
		tls := map[string]any{"enabled": true, "server_name": sni}
		if queryBool(q, "insecure", "allowInsecure", "allow_insecure") {
			tls["insecure"] = true
		}
		if alpn := parseALPN(queryFirst(q, "alpn")); len(alpn) > 0 {
			tls["alpn"] = alpn
		} else {
			tls["alpn"] = []any{"h3"}
		}
		ob["tls"] = tls
	}
	return Node{Tag: tag, Outbound: ob}, nil
}

// ─── AnyTLS ──────────────────────────────────────────────────────────────────

func parseAnyTLS(line string) (Node, error) {
	u, err := url.Parse(line)
	if err != nil {
		return Node{}, fmt.Errorf("bad anytls link: %w", err)
	}
	password := ""
	if u.User != nil {
		password = u.User.Username()
		if p, ok := u.User.Password(); ok && p != "" {
			password = u.User.Username() + ":" + p
		}
		if d, err := url.QueryUnescape(password); err == nil {
			password = d
		}
	}
	host := u.Hostname()
	port, _ := strconv.Atoi(u.Port())
	if password == "" || host == "" || port == 0 {
		return Node{}, fmt.Errorf("anytls: incomplete fields")
	}
	q := u.Query()
	tag := tagFromFragment(u.Fragment, "anytls-"+host)
	ob := map[string]any{
		"type":        "anytls",
		"tag":         tag,
		"server":      host,
		"server_port": port,
		"password":    password,
	}
	applyTLS(ob, q, "tls")
	if _, ok := ob["tls"]; !ok {
		sni := firstNonEmpty(queryFirst(q, "sni", "peer"), host)
		tls := map[string]any{"enabled": true, "server_name": sni}
		if queryBool(q, "insecure", "allowInsecure") {
			tls["insecure"] = true
		}
		ob["tls"] = tls
	}
	return Node{Tag: tag, Outbound: ob}, nil
}

// ─── WireGuard ───────────────────────────────────────────────────────────────

func parseWireGuard(line string) (Node, error) {
	// wireguard://PRIVATEKEY@server:port?publickey=&address=&dns=&mtu=&reserved=&presharedkey=&allowed_ips=
	u, err := url.Parse(line)
	if err != nil {
		return Node{}, fmt.Errorf("bad wireguard link: %w", err)
	}
	privateKey := ""
	if u.User != nil {
		privateKey = u.User.Username()
		if d, err := url.QueryUnescape(privateKey); err == nil {
			privateKey = d
		}
	}
	host := u.Hostname()
	port, _ := strconv.Atoi(u.Port())
	q := u.Query()
	if privateKey == "" {
		privateKey = queryFirst(q, "privatekey", "private_key", "privateKey")
	}
	pub := queryFirst(q, "publickey", "public_key", "publicKey", "peer_public_key")
	if privateKey == "" || host == "" || port == 0 || pub == "" {
		return Node{}, fmt.Errorf("wireguard: incomplete fields (need private key, publickey, host:port)")
	}
	tag := tagFromFragment(u.Fragment, "wg-"+host)
	addr := queryFirst(q, "address", "local_address", "ip")
	if addr == "" {
		addr = "10.0.0.2/32"
	}
	var localAddrs []any
	for _, a := range strings.Split(addr, ",") {
		a = strings.TrimSpace(a)
		if a != "" {
			localAddrs = append(localAddrs, a)
		}
	}
	ob := map[string]any{
		"type":            "wireguard",
		"tag":             tag,
		"server":          host,
		"server_port":     port,
		"local_address":   localAddrs,
		"private_key":     privateKey,
		"peer_public_key": pub,
	}
	if psk := queryFirst(q, "presharedkey", "pre_shared_key", "psk"); psk != "" {
		ob["pre_shared_key"] = psk
	}
	if mtu := queryInt(q, "mtu"); mtu > 0 {
		ob["mtu"] = mtu
	}
	if reserved := parseCSVInts(queryFirst(q, "reserved")); len(reserved) > 0 {
		ob["reserved"] = reserved
	}
	return Node{Tag: tag, Outbound: ob}, nil
}

// ─── SOCKS ───────────────────────────────────────────────────────────────────

func parseSOCKS(line string) (Node, error) {
	u, err := url.Parse(line)
	if err != nil {
		return Node{}, fmt.Errorf("bad socks link: %w", err)
	}
	host := u.Hostname()
	port, _ := strconv.Atoi(u.Port())
	if host == "" || port == 0 {
		return Node{}, fmt.Errorf("socks: incomplete fields")
	}
	tag := tagFromFragment(u.Fragment, "socks-"+host)
	ver := "5"
	lower := strings.ToLower(u.Scheme)
	switch {
	case strings.HasPrefix(lower, "socks4a"):
		ver = "4a"
	case strings.HasPrefix(lower, "socks4"):
		ver = "4"
	}
	ob := map[string]any{
		"type":        "socks",
		"tag":         tag,
		"server":      host,
		"server_port": port,
		"version":     ver,
	}
	if u.User != nil {
		ob["username"] = u.User.Username()
		if p, ok := u.User.Password(); ok {
			ob["password"] = p
		}
	}
	return Node{Tag: tag, Outbound: ob}, nil
}

// ─── HTTP proxy ──────────────────────────────────────────────────────────────

func parseHTTPProxy(line string) (Node, error) {
	u, err := url.Parse(line)
	if err != nil {
		return Node{}, fmt.Errorf("bad http link: %w", err)
	}
	if !isLikelyHTTPProxyShare(u) {
		return Node{}, fmt.Errorf("not a proxy share link (http/https subscription URLs use “导入订阅”)")
	}
	host := u.Hostname()
	port, _ := strconv.Atoi(u.Port())
	if port == 0 {
		if strings.EqualFold(u.Scheme, "https") {
			port = 443
		} else {
			port = 80
		}
	}
	if host == "" {
		return Node{}, fmt.Errorf("http: incomplete fields")
	}
	tag := tagFromFragment(u.Fragment, "http-"+host)
	ob := map[string]any{
		"type":        "http",
		"tag":         tag,
		"server":      host,
		"server_port": port,
	}
	if u.User != nil {
		ob["username"] = u.User.Username()
		if p, ok := u.User.Password(); ok {
			ob["password"] = p
		}
	}
	if strings.EqualFold(u.Scheme, "https") {
		ob["tls"] = map[string]any{
			"enabled":     true,
			"server_name": host,
		}
	}
	return Node{Tag: tag, Outbound: ob}, nil
}

// ─── Snell ───────────────────────────────────────────────────────────────────

func parseSnell(line string) (Node, error) {
	u, err := url.Parse(line)
	if err != nil {
		return Node{}, fmt.Errorf("bad snell link: %w", err)
	}
	psk := ""
	if u.User != nil {
		psk = u.User.Username()
		if d, err := url.QueryUnescape(psk); err == nil {
			psk = d
		}
	}
	host := u.Hostname()
	port, _ := strconv.Atoi(u.Port())
	q := u.Query()
	if psk == "" {
		psk = queryFirst(q, "psk", "password")
	}
	if psk == "" || host == "" || port == 0 {
		return Node{}, fmt.Errorf("snell: incomplete fields")
	}
	ver := queryInt(q, "version", "v")
	if ver == 0 {
		ver = 4
	}
	tag := tagFromFragment(u.Fragment, "snell-"+host)
	ob := map[string]any{
		"type":        "snell",
		"tag":         tag,
		"server":      host,
		"server_port": port,
		"version":     ver,
		"psk":         psk,
	}
	if uk := queryFirst(q, "userkey", "user_key"); uk != "" {
		ob["userkey"] = uk
	}
	if queryBool(q, "reuse") {
		ob["reuse"] = true
	}
	if ver >= 6 {
		if mode := queryFirst(q, "mode"); mode != "" {
			ob["mode"] = mode
		}
	} else {
		if obfs := queryFirst(q, "obfs", "obfs_mode"); obfs != "" {
			ob["obfs_mode"] = obfs
		}
		if h := queryFirst(q, "obfs-host", "obfs_host", "host"); h != "" {
			ob["obfs_host"] = h
		}
	}
	return Node{Tag: tag, Outbound: ob}, nil
}

// ─── NaiveProxy ──────────────────────────────────────────────────────────────

// looksLikeNaiveHTTPS detects the common share form:
//
//	https://user:pass@host:port#name
//
// (as opposed to a bare HTTP CONNECT proxy without credentials).
func looksLikeNaiveHTTPS(line string) bool {
	u, err := url.Parse(line)
	if err != nil || u == nil {
		return false
	}
	if !strings.EqualFold(u.Scheme, "https") {
		return false
	}
	if u.User == nil || u.User.Username() == "" {
		return false
	}
	// must look like a proxy endpoint (host + optional port, not a long subscription path)
	return isLikelyHTTPProxyShare(u)
}

func parseNaive(line string) (Node, error) {
	// Supported forms:
	//   https://user:pass@host:port#name          (common share form)
	//   naive+https://user:pass@host:port
	//   naive+quic://user:pass@host:port
	//   naive+http://user:pass@host:port
	//   naive://user:pass@host:port               (assume https)
	normalized := line
	lower := strings.ToLower(line)
	quic := false
	switch {
	case strings.HasPrefix(lower, "naive+https://"):
		normalized = "https://" + line[len("naive+https://"):]
	case strings.HasPrefix(lower, "naive+http://"):
		normalized = "http://" + line[len("naive+http://"):]
	case strings.HasPrefix(lower, "naive+quic://"):
		normalized = "https://" + line[len("naive+quic://"):]
		quic = true
	case strings.HasPrefix(lower, "naive://"):
		normalized = "https://" + line[len("naive://"):]
	case strings.HasPrefix(lower, "https://"):
		// already https://user:pass@host — keep as-is
		normalized = line
	}
	u, err := url.Parse(normalized)
	if err != nil {
		return Node{}, fmt.Errorf("bad naive link: %w", err)
	}
	host := u.Hostname()
	port, _ := strconv.Atoi(u.Port())
	if port == 0 {
		if strings.EqualFold(u.Scheme, "http") {
			port = 80
		} else {
			port = 443
		}
	}
	if host == "" {
		return Node{}, fmt.Errorf("naive: incomplete fields")
	}
	if u.User == nil || u.User.Username() == "" {
		return Node{}, fmt.Errorf("naive: missing username")
	}
	tag := tagFromFragment(u.Fragment, "naive-"+host)
	ob := map[string]any{
		"type":        "naive",
		"tag":         tag,
		"server":      host,
		"server_port": port,
		"username":    u.User.Username(),
	}
	if p, ok := u.User.Password(); ok {
		ob["password"] = p
	}
	if quic {
		ob["quic"] = true
	}
	// TLS for https / quic (naive over plain http is rare but allowed without tls block)
	if !strings.EqualFold(u.Scheme, "http") || quic {
		sni := firstNonEmpty(u.Query().Get("sni"), host)
		ob["tls"] = map[string]any{
			"enabled":     true,
			"server_name": sni,
		}
	}
	return Node{Tag: tag, Outbound: ob}, nil
}

// ─── SSH ─────────────────────────────────────────────────────────────────────

func parseSSH(line string) (Node, error) {
	u, err := url.Parse(line)
	if err != nil {
		return Node{}, fmt.Errorf("bad ssh link: %w", err)
	}
	host := u.Hostname()
	port, _ := strconv.Atoi(u.Port())
	if port == 0 {
		port = 22
	}
	if host == "" {
		return Node{}, fmt.Errorf("ssh: incomplete fields")
	}
	tag := tagFromFragment(u.Fragment, "ssh-"+host)
	user := "root"
	password := ""
	if u.User != nil {
		if u.User.Username() != "" {
			user = u.User.Username()
		}
		password, _ = u.User.Password()
	}
	q := u.Query()
	if password == "" {
		password = queryFirst(q, "password", "pass")
	}
	ob := map[string]any{
		"type":        "ssh",
		"tag":         tag,
		"server":      host,
		"server_port": port,
		"user":        user,
	}
	if password != "" {
		ob["password"] = password
	}
	if pk := queryFirst(q, "private_key", "privatekey", "key"); pk != "" {
		ob["private_key"] = pk
	}
	if pkp := queryFirst(q, "private_key_path", "key_path"); pkp != "" {
		ob["private_key_path"] = pkp
	}
	return Node{Tag: tag, Outbound: ob}, nil
}

// ─── small any helpers for JSON vmess ────────────────────────────────────────

func anyString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case float64:
		// json numbers
		if t == float64(int64(t)) {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'f', -1, 64)
	case json.Number:
		return t.String()
	case bool:
		if t {
			return "true"
		}
		return "false"
	default:
		return ""
	}
}

func anyInt(v any) int {
	switch t := v.(type) {
	case float64:
		return int(t)
	case int:
		return t
	case string:
		n, _ := strconv.Atoi(strings.TrimSpace(t))
		return n
	case json.Number:
		n, _ := t.Int64()
		return int(n)
	default:
		return 0
	}
}

func anyBool(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		switch strings.ToLower(t) {
		case "1", "true", "yes", "on":
			return true
		}
	case float64:
		return t != 0
	}
	return false
}

func cloneValues(q url.Values) url.Values {
	out := make(url.Values, len(q))
	for k, vs := range q {
		cp := make([]string, len(vs))
		copy(cp, vs)
		out[k] = cp
	}
	return out
}
