package sharelink

import (
	"encoding/base64"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// UUID accepted by sing-box (standard form, or 32-hex without dashes).
var (
	uuidDashedRe = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	uuidHex32Re  = regexp.MustCompile(`(?i)^[0-9a-f]{32}$`)
)

// normalizeUUID trims and accepts standard UUID or 32-hex form.
// Returns canonical lowercase dashed form, or error if invalid.
func normalizeUUID(s string) (string, error) {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, "{}")
	s = strings.ToLower(s)
	if s == "" {
		return "", fmt.Errorf("empty uuid")
	}
	if uuidDashedRe.MatchString(s) {
		return s, nil
	}
	// some panels export id without dashes
	if uuidHex32Re.MatchString(s) {
		return s[0:8] + "-" + s[8:12] + "-" + s[12:16] + "-" + s[16:20] + "-" + s[20:32], nil
	}
	return "", fmt.Errorf("invalid uuid %q", s)
}

// splitUUIDPassword handles common TUIC mistakes where uuid and password are
// glued as "uuid:password" in a single field (or URL-encoded as one username).
// Returns (uuid, passwordExtra). passwordExtra is empty if no split applied.
func splitUUIDPassword(raw string) (uuidPart, passwordPart string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", ""
	}
	// Already a pure UUID — nothing to split.
	if _, err := normalizeUUID(raw); err == nil {
		return raw, ""
	}
	// uuid:password (password may contain ':')
	if i := strings.Index(raw, ":"); i > 0 {
		left, right := raw[:i], raw[i+1:]
		if _, err := normalizeUUID(left); err == nil && right != "" {
			return left, right
		}
	}
	return raw, ""
}

func decodeBase64(s string) (string, error) {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "-", "+")
	s = strings.ReplaceAll(s, "_", "/")
	// strip whitespace sometimes present in multi-line base64
	s = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == ' ' || r == '\t' {
			return -1
		}
		return r
	}, s)
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

func decodeBase64Bytes(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "-", "+")
	s = strings.ReplaceAll(s, "_", "/")
	s = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == ' ' || r == '\t' {
			return -1
		}
		return r
	}, s)
	for len(s)%4 != 0 {
		s += "="
	}
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		b, err = base64.RawStdEncoding.DecodeString(strings.TrimRight(s, "="))
		if err != nil {
			return nil, err
		}
	}
	return b, nil
}

func tagFromFragment(frag, fallback string) string {
	if frag == "" {
		return sanitizeTag(fallback)
	}
	if u, err := url.QueryUnescape(frag); err == nil {
		frag = u
	}
	frag = strings.TrimSpace(frag)
	if frag == "" {
		return sanitizeTag(fallback)
	}
	return sanitizeTag(frag)
}

func sanitizeTag(s string) string {
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
		return "", 0, err
	}
	port, err := strconv.Atoi(portStr)
	return host, port, err
}

func queryFirst(q url.Values, keys ...string) string {
	for _, k := range keys {
		if v := strings.TrimSpace(q.Get(k)); v != "" {
			return v
		}
	}
	return ""
}

func queryBool(q url.Values, keys ...string) bool {
	v := strings.ToLower(queryFirst(q, keys...))
	switch v {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func queryInt(q url.Values, keys ...string) int {
	v := queryFirst(q, keys...)
	if v == "" {
		return 0
	}
	n, _ := strconv.Atoi(v)
	return n
}

func parsePort(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return n
}

func parseALPN(s string) []any {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	// accept comma or percent-encoded comma already decoded by url.Query
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return r == ',' || r == ';'
	})
	var out []any
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func parseCSVInts(s string) []any {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	var out []any
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		n, err := strconv.Atoi(p)
		if err != nil {
			return nil
		}
		out = append(out, n)
	}
	return out
}

func splitUserPass(userinfo string) (user, pass string) {
	if i := strings.Index(userinfo, ":"); i >= 0 {
		return userinfo[:i], userinfo[i+1:]
	}
	return userinfo, ""
}

// isLikelyHTTPProxyShare decides if an http(s) URL is a proxy share link
// rather than a subscription / webpage URL.
func isLikelyHTTPProxyShare(u *url.URL) bool {
	if u == nil || u.Host == "" {
		return false
	}
	// subscription-style paths are not node share links
	path := strings.TrimSpace(u.Path)
	if path != "" && path != "/" {
		return false
	}
	// need an explicit port for bare host links
	if u.Port() == "" && u.User == nil {
		return false
	}
	// query-heavy URLs (token=…) are usually subscriptions
	if len(u.RawQuery) > 64 {
		return false
	}
	return true
}

func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if strings.TrimSpace(s) != "" {
			return s
		}
	}
	return ""
}
