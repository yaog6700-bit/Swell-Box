package config

import (
	"fmt"
	"strings"
)

// sanitizeOutboundGroups fills empty selector/urltest outbounds so sing-box
// does not fail with "missing tags". Intended for **runtime** config only —
// user template files on disk should stay as the user provided them.
//
// Prefers a direct outbound as fallback, otherwise the first leaf proxy tag.
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
		if fallback == "" {
			stillEmpty = append(stillEmpty, fmt.Sprintf("%s[%s]", t, tag))
			continue
		}
		m["outbounds"] = []any{fallback}
		// selector supports default; urltest does not (strict JSON schema in newer sing-box).
		if t == "selector" {
			if def, _ := m["default"].(string); def == "" {
				m["default"] = fallback
			}
		}
		warns = append(warns, fmt.Sprintf("%s[%s] empty → %s (runtime only)", t, tag, fallback))
	}

	if len(stillEmpty) > 0 {
		return warns, fmt.Errorf(
			"empty %s and no direct/proxy to use as fallback — add nodes (e.g. via subscription) or a direct outbound",
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
		if t == "direct" || strings.EqualFold(tag, "direct") {
			return tag
		}
		if t == "selector" || t == "urltest" || t == "block" || t == "dns" {
			continue
		}
		if firstLeaf == "" {
			firstLeaf = tag
		}
	}
	return firstLeaf
}

// inspectEmptyGroups reports empty selector/urltest groups without modifying root.
// Used at import time so we can warn without rewriting the user's template.
func inspectEmptyGroups(root map[string]any) (empty []string, hasFallback bool) {
	outbounds, _ := root["outbounds"].([]any)
	hasFallback = pickFallbackOutbound(outbounds) != ""
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
		n := 0
		for _, s := range list {
			if strings.TrimSpace(s) != "" {
				n++
			}
		}
		if n == 0 {
			tag, _ := m["tag"].(string)
			empty = append(empty, fmt.Sprintf("%s[%s]", t, tag))
		}
	}
	return empty, hasFallback
}
