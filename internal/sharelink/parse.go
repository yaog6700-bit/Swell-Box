package sharelink

import (
	"encoding/base64"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

// Node is a parsed share link ready to become a sing-box outbound.
type Node struct {
	Tag      string
	Outbound map[string]any
}

// Parse detects ss:// or vless:// (and multi-line paste; first valid wins / all).
func Parse(raw string) ([]Node, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("clipboard is empty")
	}
	// Support paste of multiple links separated by newlines.
	var lines []string
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
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
		return nil, fmt.Errorf("unsupported link (need ss:// or vless://)")
	}
	return nodes, nil
}

func parseOne(line string) (Node, error) {
	lower := strings.ToLower(line)
	switch {
	case strings.HasPrefix(lower, "ss://"):
		return parseSS(line)
	case strings.HasPrefix(lower, "vless://"):
		return parseVLESS(line)
	default:
		return Node{}, fmt.Errorf("unsupported scheme (need ss:// or vless://)")
	}
}

func parseSS(line string) (Node, error) {
	// ss://base64(method:password)@host:port#name
	// or ss://base64(method:password@host:port)#name
	u, err := url.Parse(line)
	if err != nil {
		return Node{}, fmt.Errorf("bad ss link: %w", err)
	}

	var method, password, host string
	var port int

	if u.User != nil && u.Host != "" {
		// userinfo may be base64(method:password) or method with password field
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
		// entire body after ss:// may be base64
		body := strings.TrimPrefix(line, "ss://")
		body = strings.TrimPrefix(body, "SS://")
		if i := strings.Index(body, "#"); i >= 0 {
			body = body[:i]
		}
		if i := strings.Index(body, "?"); i >= 0 {
			body = body[:i]
		}
		decoded, err := decodeBase64(body)
		if err != nil {
			return Node{}, fmt.Errorf("ss body decode: %w", err)
		}
		// method:password@host:port
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
	return Node{Tag: tag, Outbound: ob}, nil
}

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
	security := strings.ToLower(q.Get("security"))
	flow := q.Get("flow")
	sni := q.Get("sni")
	if sni == "" {
		sni = q.Get("peer")
	}
	fp := q.Get("fp")
	if fp == "" {
		fp = "chrome"
	}
	pbk := q.Get("pbk")
	sid := q.Get("sid")
	network := q.Get("type")
	if network == "" {
		network = "tcp"
	}

	tag := tagFromFragment(u.Fragment, "vless-"+host)
	ob := map[string]any{
		"type":        "vless",
		"tag":         tag,
		"server":      host,
		"server_port": port,
		"uuid":        uuid,
		"packet_encoding": "xudp",
	}
	if flow != "" {
		ob["flow"] = flow
	}

	if security == "reality" || security == "tls" {
		tls := map[string]any{
			"enabled":     true,
			"server_name": sni,
			"utls": map[string]any{
				"enabled":     true,
				"fingerprint": fp,
			},
		}
		if security == "reality" {
			tls["reality"] = map[string]any{
				"enabled":    true,
				"public_key": pbk,
				"short_id":   sid,
			}
		}
		ob["tls"] = tls
	}

	// transport for non-tcp if needed later; tcp is default
	_ = network

	return Node{Tag: tag, Outbound: ob}, nil
}

func splitMethodPass(s string) (method, password string, err error) {
	i := strings.Index(s, ":")
	if i <= 0 {
		return "", "", fmt.Errorf("ss: method:password format invalid")
	}
	return s[:i], s[i+1:], nil
}

func splitHostPort(s string) (string, int, error) {
	host, portStr, err := net.SplitHostPort(s)
	if err != nil {
		// bare host without brackets for ipv4:port already handled
		return "", 0, err
	}
	port, err := strconv.Atoi(portStr)
	return host, port, err
}

func decodeBase64(s string) (string, error) {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "-", "+")
	s = strings.ReplaceAll(s, "_", "/")
	for len(s)%4 != 0 {
		s += "="
	}
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		b, err = base64.RawStdEncoding.DecodeString(strings.TrimRight(s, "="))
		if err != nil {
			return "", err
		}
	}
	return string(b), nil
}

func tagFromFragment(frag, fallback string) string {
	if frag == "" {
		return sanitizeTag(fallback)
	}
	// url-unescape
	if u, err := url.QueryUnescape(frag); err == nil {
		frag = u
	}
	// keep readable but strip crazy whitespace
	frag = strings.TrimSpace(frag)
	if frag == "" {
		return sanitizeTag(fallback)
	}
	return sanitizeTag(frag)
}

func sanitizeTag(s string) string {
	// sing-box tags: keep most unicode; just trim length and control chars
	s = strings.Map(func(r rune) rune {
		if r < 32 {
			return -1
		}
		return r
	}, s)
	s = strings.TrimSpace(s)
	if s == "" {
		return "node"
	}
	if len([]rune(s)) > 32 {
		r := []rune(s)
		s = string(r[:32])
	}
	return s
}
