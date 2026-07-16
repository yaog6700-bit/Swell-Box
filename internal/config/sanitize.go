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

// convertFilledURLTestToSelector turns non-empty urltest groups into selector
// groups in the runtime config only.
//
// Official Dashboard often treats URLTest as a single auto-pick unit and does
// not let users open the group to choose among 🚜 / node-A / node-B. Selector
// groups show the member list so users can pick a specific node. Disk template
// stays urltest.
func convertFilledURLTestToSelector(root map[string]any) {
	outbounds, _ := root["outbounds"].([]any)
	for _, item := range outbounds {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if t, _ := m["type"].(string); t != "urltest" {
			continue
		}
		members := toStringSlice(m["outbounds"])
		var clean []string
		for _, s := range members {
			s = strings.TrimSpace(s)
			if s != "" {
				clean = append(clean, s)
			}
		}
		if len(clean) == 0 {
			continue // empty groups handled by sanitize
		}
		m["type"] = "selector"
		m["outbounds"] = toAnySlice(clean)
		// urltest used "url" for latency; drop fields selector ignores / rejects
		delete(m, "url")
		delete(m, "interval")
		delete(m, "tolerance")
		delete(m, "idle_timeout")
		delete(m, "interrupt_exist_connections")
		if def, _ := m["default"].(string); def == "" {
			m["default"] = clean[0]
		}
	}
}

// exposeNestedLeavesInSelectors appends leaf proxies that currently sit only
// under region urltest/selector groups onto parent selectors (e.g. Manual).
// Runtime-only: helps official Dashboard show real nodes without requiring
// users to open nested Singapore → node. Disk template is unchanged.
func exposeNestedLeavesInSelectors(root map[string]any) {
	outbounds, _ := root["outbounds"].([]any)
	if len(outbounds) == 0 {
		return
	}
	type info struct {
		typ     string
		members []string
	}
	byTag := map[string]*info{}
	for _, item := range outbounds {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		tag, _ := m["tag"].(string)
		if tag == "" {
			continue
		}
		t, _ := m["type"].(string)
		inf := &info{typ: t}
		if t == "selector" || t == "urltest" {
			inf.members = toStringSlice(m["outbounds"])
		}
		byTag[tag] = inf
	}

	// Collect leaves nested under each direct member of a selector.
	leavesUnder := func(member string) []string {
		inf, ok := byTag[member]
		if !ok {
			return nil
		}
		if inf.typ != "selector" && inf.typ != "urltest" {
			return nil
		}
		var leaves []string
		for _, c := range inf.members {
			if c == "" || isJunkOutboundTag(c) {
				continue
			}
			ci, ok := byTag[c]
			if !ok {
				// unknown → treat as leaf name
				leaves = append(leaves, c)
				continue
			}
			if ci.typ == "selector" || ci.typ == "urltest" || ci.typ == "direct" || ci.typ == "block" || ci.typ == "dns" {
				continue
			}
			leaves = append(leaves, c)
		}
		return leaves
	}

	for _, item := range outbounds {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if t, _ := m["type"].(string); t != "selector" {
			continue
		}
		// Skip huge policy selectors that already list many leaves; only
		// expand groups that look like "region wrappers" (Manual-style).
		tag, _ := m["tag"].(string)
		members := toStringSlice(m["outbounds"])
		if len(members) == 0 {
			continue
		}
		// Only expand if at least one member is a nested group with leaves.
		var extra []string
		seen := map[string]bool{}
		for _, mem := range members {
			seen[mem] = true
		}
		for _, mem := range members {
			for _, leaf := range leavesUnder(mem) {
				if seen[leaf] {
					continue
				}
				seen[leaf] = true
				extra = append(extra, leaf)
			}
		}
		if len(extra) == 0 {
			continue
		}
		// Prepend leaves so they appear first in Dashboard / Clash UI.
		newList := make([]string, 0, len(extra)+len(members))
		newList = append(newList, extra...)
		newList = append(newList, members...)
		m["outbounds"] = toAnySlice(newList)
		_ = tag
	}
}

func isJunkOutboundTag(s string) bool {
	low := strings.ToLower(strings.TrimSpace(s))
	return low == "" || low == "direct" || low == "block" || low == "dns" || low == "reject"
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
