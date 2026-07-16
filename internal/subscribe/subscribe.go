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

// FetchURL downloads a subscription URL and parses share links / Clash configs.
// Supports:
//   - Clash / Clash Meta YAML or JSON (proxies: …) — multi-protocol including snell
//   - plain multi-line share links (ss/vmess/vless/trojan/hysteria2/tuic/snell/…)
//   - base64-encoded URI lists (v2rayN style)
func FetchURL(rawURL string) ([]sharelink.Node, error) {
	rawURL = strings.TrimSpace(rawURL)
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		return nil, fmt.Errorf("subscription must be http(s) URL")
	}

	body, err := downloadSub(rawURL)
	if err != nil {
		return nil, err
	}
	return ParseBody(body)
}

func downloadSub(rawURL string) (string, error) {
	client := &http.Client{Timeout: 45 * time.Second}

	// Many panels return different formats by User-Agent:
	//   Clash*  → full multi-protocol YAML (ss + snell + vless + …)
	//   v2rayN  → base64 URI list (often only ss/vmess/vless/trojan)
	// Prefer Clash Meta first so snell / anytls / … are not dropped.
	uas := []string{
		"ClashMeta/1.18.0",
		"clash.meta",
		"ClashForWindows/0.20.39",
		"v2rayN/6.45",
		"Swell-Box/0.2.11",
	}

	var lastErr error
	var bestBody string
	var bestScore int

	for _, ua := range uas {
		req, err := http.NewRequest(http.MethodGet, rawURL, nil)
		if err != nil {
			return "", err
		}
		req.Header.Set("User-Agent", ua)
		req.Header.Set("Accept", "*/*")
		resp, err := client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("download: %w", err)
			continue
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20)) // 8MB
		resp.Body.Close()
		if err != nil {
			lastErr = err
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			lastErr = fmt.Errorf("http %d", resp.StatusCode)
			continue
		}
		text := string(body)
		if strings.TrimSpace(text) == "" {
			lastErr = fmt.Errorf("empty subscription")
			continue
		}
		// Score by how many nodes we can parse — pick richest response.
		if nodes, err := ParseBody(text); err == nil && len(nodes) > bestScore {
			bestScore = len(nodes)
			bestBody = text
			// Clash YAML with many nodes is almost always the best; stop early.
			if bestScore >= 3 && looksRich(text) {
				return bestBody, nil
			}
		} else if bestBody == "" {
			bestBody = text // keep last non-empty body as fallback
		}
	}

	if bestBody != "" {
		return bestBody, nil
	}
	if lastErr != nil {
		return "", lastErr
	}
	return "", fmt.Errorf("empty subscription")
}

func looksRich(body string) bool {
	low := strings.ToLower(body)
	return strings.Contains(low, "proxies:") || strings.Contains(low, `"proxies"`) ||
		strings.Count(body, "://") >= 3
}

// ParseBody extracts nodes from subscription body text.
func ParseBody(body string) ([]sharelink.Node, error) {
	body = strings.TrimSpace(body)
	body = strings.TrimPrefix(body, "\ufeff")
	if body == "" {
		return nil, fmt.Errorf("empty subscription")
	}

	// 1) Clash YAML/JSON first — this is what multi-protocol panels actually serve.
	//    Previously only URI lines were tried, so snell/vless/… inside Clash configs
	//    were silently dropped while a few ss:// leftovers still imported.
	if nodes, err := sharelink.ParseClashConfig(body); err == nil && len(nodes) > 0 {
		return nodes, nil
	}

	// 2) Plain multi-line share links / Clash classical one-liners.
	if nodes, err := sharelink.Parse(body); err == nil && len(nodes) > 0 {
		// If body looks like Clash but Parse only got a few URI leftovers,
		// still prefer whatever we got; Clash path already failed above.
		return nodes, nil
	}

	// 3) Whole-body base64 (v2rayN style) → re-run Clash + URI parsers.
	if decoded, err := decodeBase64Flexible(body); err == nil {
		decoded = strings.TrimSpace(decoded)
		decoded = strings.TrimPrefix(decoded, "\ufeff")
		if nodes, err := sharelink.ParseClashConfig(decoded); err == nil && len(nodes) > 0 {
			return nodes, nil
		}
		if nodes, err := sharelink.Parse(decoded); err == nil && len(nodes) > 0 {
			return nodes, nil
		}
		// Line-by-line after decode (some lines may be individually base64)
		if all := parseLines(decoded); len(all) > 0 {
			return all, nil
		}
	}

	// 4) Line-by-line original body
	if all := parseLines(body); len(all) > 0 {
		return all, nil
	}

	return nil, fmt.Errorf("no supported share links found in subscription (ss/vmess/vless/trojan/hysteria2/tuic/snell/… or Clash proxies)")
}

func parseLines(body string) []sharelink.Node {
	var all []sharelink.Node
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Per-line base64 (rare)
		if !strings.Contains(line, "://") && !strings.Contains(line, "=") {
			if decoded, err := decodeBase64Flexible(line); err == nil {
				decoded = strings.TrimSpace(decoded)
				if ns, err := sharelink.Parse(decoded); err == nil {
					all = append(all, ns...)
					continue
				}
			}
		}
		ns, err := sharelink.Parse(line)
		if err != nil {
			continue
		}
		all = append(all, ns...)
	}
	return all
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
