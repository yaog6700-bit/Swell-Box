package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// SelectorInfo is a selector outbound and its members.
type SelectorInfo struct {
	Tag       string
	Default   string
	Outbounds []string
}

// SwitchableNode is a leaf proxy shown in the tray node menu.
// Group is the selector/urltest to call Clash Select on (may differ from the top-level Manual group).
type SwitchableNode struct {
	Tag   string
	Group string
}

// ListSelectors reads selector outbounds from a user config file.
func ListSelectors(configPath string) ([]SelectorInfo, error) {
	raw, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}
	var root map[string]any
	if err := json.Unmarshal(raw, &root); err != nil {
		return nil, err
	}
	outbounds, _ := root["outbounds"].([]any)
	var list []SelectorInfo
	for _, item := range outbounds {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if m["type"] != "selector" {
			continue
		}
		tag, _ := m["tag"].(string)
		def, _ := m["default"].(string)
		var members []string
		for _, o := range toStringSlice(m["outbounds"]) {
			if o == "direct" || o == "block" || o == "dns" {
				continue
			}
			members = append(members, o)
		}
		if tag == "" {
			continue
		}
		list = append(list, SelectorInfo{Tag: tag, Default: def, Outbounds: members})
	}
	return list, nil
}

// ListSwitchableNodes returns leaf proxies for the tray「节点」menu.
//
// Product rule: only the default profile config.json is managed here
// (import node / subscription + selector "proxy"). Any other active file
// (imported full templates) returns empty — switch nodes in Dashboard.
// Nested groups under "proxy" are NOT expanded into this menu.
func ListSwitchableNodes(configPath string) (primary string, nodes []SwitchableNode, current string, err error) {
	// Enforce home profile only (by file name).
	if base := filepath.Base(configPath); !IsDefaultConfigName(base) {
		return "", nil, "", nil
	}
	raw, err := os.ReadFile(configPath)
	if err != nil {
		return "", nil, "", err
	}
	var root map[string]any
	if err := json.Unmarshal(raw, &root); err != nil {
		return "", nil, "", err
	}
	outbounds, _ := root["outbounds"].([]any)

	type obInfo struct {
		Type    string
		Members []string
		Default string
	}
	byTag := map[string]obInfo{}
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
		info := obInfo{Type: t}
		if t == "selector" || t == "urltest" {
			info.Members = toStringSlice(m["outbounds"])
			info.Default, _ = m["default"].(string)
		}
		byTag[tag] = info
	}

	// Only the canonical Swell-Box group.
	const proxyTag = "proxy"
	p, ok := byTag[proxyTag]
	if !ok || p.Type != "selector" {
		// Imported full template or no proxy group → tray node list stays empty.
		return "", nil, "", nil
	}
	primary = proxyTag
	current = p.Default

	for _, m := range p.Members {
		if isJunkTag(m) {
			continue
		}
		info, ok := byTag[m]
		if ok && (info.Type == "selector" || info.Type == "urltest") {
			// Nested policy groups are not shown in 节点 menu.
			continue
		}
		// Leaf (or unknown tag treated as leaf) under proxy.
		if ok && !isLeafProxy(info.Type) && info.Type != "" {
			continue
		}
		nodes = append(nodes, SwitchableNode{Tag: m, Group: primary})
	}
	return primary, nodes, current, nil
}

func isLeafProxy(typ string) bool {
	switch typ {
	case "", "selector", "urltest", "direct", "block", "dns", "blackhole":
		return false
	default:
		return true
	}
}

func isJunkTag(s string) bool {
	low := strings.ToLower(strings.TrimSpace(s))
	return low == "" || low == "direct" || low == "block" || low == "dns" || low == "reject"
}

// SetSelectorDefault updates selector default in the user config file on disk.
func SetSelectorDefault(configPath, selectorTag, nodeTag string) error {
	raw, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}
	var root map[string]any
	if err := json.Unmarshal(raw, &root); err != nil {
		return err
	}
	outbounds, _ := root["outbounds"].([]any)
	for i, item := range outbounds {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if m["type"] == "selector" && m["tag"] == selectorTag {
			m["default"] = nodeTag
			outbounds[i] = m
			break
		}
	}
	root["outbounds"] = outbounds
	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath, out, 0o644)
}
