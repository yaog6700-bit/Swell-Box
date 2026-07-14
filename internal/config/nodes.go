package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/swell-app/swellbox/internal/sharelink"
)

// AddNodesToActiveConfig appends parsed nodes into the active user config.
// Ensures a selector named "proxy" lists them; sets default to the last imported.
func AddNodesToActiveConfig(settings *AppSettings, nodes []sharelink.Node) (tags []string, err error) {
	path, err := ActiveConfigPath(settings)
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var root map[string]any
	if err := json.Unmarshal(raw, &root); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	outbounds, _ := root["outbounds"].([]any)
	existing := map[string]int{} // tag -> index in outbounds
	for i, item := range outbounds {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if t, _ := m["tag"].(string); t != "" {
			existing[t] = i
		}
	}

	// Find or create selector "proxy"
	var selector map[string]any
	selIdx := -1
	for i, item := range outbounds {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if m["type"] == "selector" && m["tag"] == "proxy" {
			selector = m
			selIdx = i
			break
		}
	}
	if selector == nil {
		selector = map[string]any{
			"type":      "selector",
			"tag":       "proxy",
			"outbounds": []any{"direct"},
		}
		// put selector first
		outbounds = append([]any{selector}, outbounds...)
		selIdx = 0
		// ensure direct exists
		if _, ok := existing["direct"]; !ok {
			outbounds = append(outbounds, map[string]any{"type": "direct", "tag": "direct"})
			existing["direct"] = len(outbounds) - 1
		}
	}

	selList := toStringSlice(selector["outbounds"])

	for _, n := range nodes {
		tag := uniqueTag(n.Tag, existing)
		n.Outbound["tag"] = tag
		if idx, ok := existing[tag]; ok {
			outbounds[idx] = n.Outbound
		} else {
			// insert after selector, before trailing direct if possible
			outbounds = append(outbounds, n.Outbound)
			existing[tag] = len(outbounds) - 1
		}
		if !contains(selList, tag) {
			// keep direct at end
			var next []string
			for _, t := range selList {
				if t != "direct" {
					next = append(next, t)
				}
			}
			next = append(next, tag, "direct")
			selList = next
		}
		tags = append(tags, tag)
	}

	if len(tags) > 0 {
		selector["outbounds"] = toAnySlice(selList)
		selector["default"] = tags[len(tags)-1]
		outbounds[selIdx] = selector
	}
	root["outbounds"] = outbounds

	// route.final -> proxy if missing
	if route, ok := root["route"].(map[string]any); ok {
		if route["final"] == nil || route["final"] == "" {
			route["final"] = "proxy"
		}
		root["route"] = route
	} else {
		root["route"] = map[string]any{
			"final":                 "proxy",
			"auto_detect_interface": true,
		}
	}

	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, out, 0o644); err != nil {
		return nil, err
	}
	return tags, nil
}

func uniqueTag(base string, existing map[string]int) string {
	base = strings.TrimSpace(base)
	if base == "" {
		base = "node"
	}
	if _, ok := existing[base]; !ok {
		return base
	}
	for i := 2; i < 1000; i++ {
		t := fmt.Sprintf("%s-%d", base, i)
		if _, ok := existing[t]; !ok {
			return t
		}
	}
	return fmt.Sprintf("%s-%d", base, len(existing)+1)
}

func toStringSlice(v any) []string {
	arr, ok := v.([]any)
	if !ok {
		if ss, ok := v.([]string); ok {
			return ss
		}
		return nil
	}
	var out []string
	for _, item := range arr {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func toAnySlice(ss []string) []any {
	out := make([]any, len(ss))
	for i, s := range ss {
		out[i] = s
	}
	return out
}

func contains(ss []string, t string) bool {
	for _, s := range ss {
		if s == t {
			return true
		}
	}
	return false
}
