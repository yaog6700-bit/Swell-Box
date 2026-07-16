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

// RemoveNodeFromConfig deletes a leaf outbound by tag from the active config,
// removes it from all selector member lists, and fixes selector defaults.
// Built-in tags (direct/block/dns/reject) cannot be removed.
func RemoveNodeFromConfig(settings *AppSettings, tag string) error {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return fmt.Errorf("empty tag")
	}
	low := strings.ToLower(tag)
	if low == "direct" || low == "block" || low == "dns" || low == "reject" {
		return fmt.Errorf("cannot remove built-in outbound %q", tag)
	}
	path, err := ActiveConfigPath(settings)
	if err != nil {
		return err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var root map[string]any
	if err := json.Unmarshal(raw, &root); err != nil {
		return fmt.Errorf("parse config: %w", err)
	}

	outbounds, _ := root["outbounds"].([]any)
	var kept []any
	removed := false
	for _, item := range outbounds {
		m, ok := item.(map[string]any)
		if !ok {
			kept = append(kept, item)
			continue
		}
		t, _ := m["type"].(string)
		// Never drop selectors / groups — only strip the tag from their lists.
		if t == "selector" || t == "urltest" {
			if list := toStringSlice(m["outbounds"]); len(list) > 0 {
				var next []string
				for _, o := range list {
					if o != tag {
						next = append(next, o)
					}
				}
				m["outbounds"] = toAnySlice(next)
				if def, _ := m["default"].(string); def == tag {
					// Pick first remaining non-empty member, prefer non-direct.
					newDef := ""
					for _, o := range next {
						if o != "" && strings.ToLower(o) != "direct" {
							newDef = o
							break
						}
					}
					if newDef == "" && len(next) > 0 {
						newDef = next[0]
					}
					if newDef != "" {
						m["default"] = newDef
					} else {
						delete(m, "default")
					}
				}
			}
			kept = append(kept, m)
			continue
		}
		if name, _ := m["tag"].(string); name == tag {
			removed = true
			continue
		}
		kept = append(kept, item)
	}
	if !removed {
		// Still OK if it was only a dangling selector reference.
		// But report if nothing referenced it either.
		foundInSelector := false
		for _, item := range outbounds {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			t, _ := m["type"].(string)
			if t != "selector" && t != "urltest" {
				continue
			}
			for _, o := range toStringSlice(m["outbounds"]) {
				if o == tag {
					foundInSelector = true
					break
				}
			}
		}
		if !foundInSelector {
			return fmt.Errorf("node %q not found", tag)
		}
	}
	root["outbounds"] = kept
	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0o644)
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
