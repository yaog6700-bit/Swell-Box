package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/swell-app/swellbox/internal/paths"
)

var safeName = regexp.MustCompile(`[^a-zA-Z0-9._\-\p{L}]+`)

// ImportResult is returned after a successful config file import.
type ImportResult struct {
	Name     string   // saved file name under data dir
	Warnings []string // auto-fixes / notes for the user
}

// ImportConfigFile copies a user-selected JSON into the data dir as config*.json,
// validates it is JSON object, repairs empty selector/urltest groups when possible,
// sets it active, and returns the saved file name + warnings.
func ImportConfigFile(settings *AppSettings, srcPath string) (ImportResult, error) {
	var zero ImportResult
	srcPath = strings.TrimSpace(srcPath)
	if srcPath == "" {
		return zero, fmt.Errorf("empty path")
	}
	raw, err := os.ReadFile(srcPath)
	if err != nil {
		return zero, fmt.Errorf("read: %w", err)
	}
	var root map[string]any
	if err := json.Unmarshal(raw, &root); err != nil {
		return zero, fmt.Errorf("not valid JSON: %w", err)
	}
	// Light sanity: sing-box configs are objects; prefer having outbounds or inbounds.
	if len(root) == 0 {
		return zero, fmt.Errorf("empty config object")
	}

	warns, err := sanitizeOutboundGroups(root)
	if err != nil {
		return zero, err
	}

	dir, err := paths.ConfigDir()
	if err != nil {
		return zero, err
	}
	base := filepath.Base(srcPath)
	base = strings.TrimSuffix(base, filepath.Ext(base))
	base = safeName.ReplaceAllString(base, "_")
	base = strings.Trim(base, "._-")
	if base == "" || strings.EqualFold(base, "app") {
		base = "imported"
	}
	// Always store as config.<name>.json so it shows in Configs menu.
	name := "config." + base + ".json"
	if name == "config.config.json" {
		name = "config.imported.json"
	}
	dst := filepath.Join(dir, name)
	// Avoid clobber without suffix
	if _, err := os.Stat(dst); err == nil {
		for i := 2; i < 100; i++ {
			cand := fmt.Sprintf("config.%s-%d.json", base, i)
			p := filepath.Join(dir, cand)
			if _, err := os.Stat(p); os.IsNotExist(err) {
				name = cand
				dst = p
				break
			}
		}
	}

	// Pretty-print for readability
	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return zero, err
	}
	if err := os.WriteFile(dst, out, 0o644); err != nil {
		return zero, err
	}

	settings.ActiveConfig = name
	if err := SaveAppSettings(settings); err != nil {
		return ImportResult{Name: name, Warnings: warns}, err
	}
	return ImportResult{Name: name, Warnings: warns}, nil
}

// sanitizeOutboundGroups fills empty selector/urltest outbounds so sing-box
// does not fail with "missing tags". Prefers a direct outbound as fallback,
// otherwise the first leaf proxy tag.
func sanitizeOutboundGroups(root map[string]any) ([]string, error) {
	outbounds, _ := root["outbounds"].([]any)
	if len(outbounds) == 0 {
		return nil, nil
	}

	fallback := pickFallbackOutbound(outbounds)
	var warns []string
	var stillEmpty []string

	for _, item := range outbounds {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		t, _ := m["type"].(string)
		if t != "selector" && t != "urltest" {
			continue
		}
		list := toStringSlice(m["outbounds"])
		// Drop blanks
		var clean []string
		for _, s := range list {
			s = strings.TrimSpace(s)
			if s != "" {
				clean = append(clean, s)
			}
		}
		tag, _ := m["tag"].(string)
		if len(clean) > 0 {
			m["outbounds"] = toAnySlice(clean)
			continue
		}
		// Empty group
		if fallback == "" {
			stillEmpty = append(stillEmpty, fmt.Sprintf("%s[%s]", t, tag))
			continue
		}
		m["outbounds"] = []any{fallback}
		if def, _ := m["default"].(string); def == "" {
			m["default"] = fallback
		}
		warns = append(warns, fmt.Sprintf("%s[%s] empty → %s", t, tag, fallback))
	}

	if len(stillEmpty) > 0 {
		return warns, fmt.Errorf(
			"empty %s (no nodes to fill). Add proxies or remove empty groups — sing-box reports: missing tags",
			strings.Join(stillEmpty, ", "),
		)
	}
	return warns, nil
}

func pickFallbackOutbound(outbounds []any) string {
	var firstLeaf string
	for _, item := range outbounds {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		t, _ := m["type"].(string)
		tag, _ := m["tag"].(string)
		if tag == "" {
			continue
		}
		// Prefer explicit direct
		if t == "direct" || strings.EqualFold(tag, "direct") {
			return tag
		}
		// Skip groups
		if t == "selector" || t == "urltest" || t == "block" || t == "dns" {
			continue
		}
		if firstLeaf == "" {
			firstLeaf = tag
		}
	}
	return firstLeaf
}
