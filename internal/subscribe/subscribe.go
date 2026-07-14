package subscribe

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/swell-app/swellbox/internal/sharelink"
)

// FetchURL downloads a subscription URL and parses share links (ss/vless).
// Supports plain text lists and common base64-encoded v2ray-style subscriptions.
func FetchURL(rawURL string) ([]sharelink.Node, error) {
	rawURL = strings.TrimSpace(rawURL)
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		return nil, fmt.Errorf("subscription must be http(s) URL")
	}

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Swell-Box/0.2")
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("http %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20)) // 8MB max
	if err != nil {
		return nil, err
	}
	return ParseBody(string(body))
}

// ParseBody extracts nodes from subscription body text.
func ParseBody(body string) ([]sharelink.Node, error) {
	body = strings.TrimSpace(body)
	if body == "" {
		return nil, fmt.Errorf("empty subscription")
	}

	// Try as plain multi-line share links first.
	if nodes, err := sharelink.Parse(body); err == nil && len(nodes) > 0 {
		return nodes, nil
	}

	// Try base64 whole body (v2ray subscription style).
	if decoded, err := decodeBase64Flexible(body); err == nil {
		decoded = strings.TrimSpace(decoded)
		if nodes, err := sharelink.Parse(decoded); err == nil && len(nodes) > 0 {
			return nodes, nil
		}
		// Line-by-line after decode
		var all []sharelink.Node
		for _, line := range strings.Split(decoded, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			ns, err := sharelink.Parse(line)
			if err != nil {
				continue
			}
			all = append(all, ns...)
		}
		if len(all) > 0 {
			return all, nil
		}
	}

	// Line-by-line original body (some lines may be base64?)
	var all []sharelink.Node
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		ns, err := sharelink.Parse(line)
		if err != nil {
			continue
		}
		all = append(all, ns...)
	}
	if len(all) == 0 {
		return nil, fmt.Errorf("no ss:// or vless:// nodes found in subscription")
	}
	return all, nil
}

func decodeBase64Flexible(s string) (string, error) {
	s = strings.TrimSpace(s)
	// strip whitespace inside
	s = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == ' ' || r == '\t' {
			return -1
		}
		return r
	}, s)
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
