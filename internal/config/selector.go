package config

import (
	"encoding/json"
	"os"
)

// SelectorInfo is a selector outbound and its members.
type SelectorInfo struct {
	Tag      string
	Default  string
	Outbounds []string
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
