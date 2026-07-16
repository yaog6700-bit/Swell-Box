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
	Warnings []string // notes only — user template is NOT rewritten
}

// ImportConfigFile copies a user-selected JSON into the data dir as config*.json.
//
// Policy: keep the user's template as-is (other apps can use the same file).
// Compatibility fixes (empty urltest groups, TUN strip, API inject, …) run only
// on the runtime copy in PrepareRuntimeConfig — never mutate the stored template
// beyond pretty-printing the original object.
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
	if len(root) == 0 {
		return zero, fmt.Errorf("empty config object")
	}

	// Warn only — do not rewrite empty groups into the saved file.
	var warns []string
	if empty, hasFB := inspectEmptyGroups(root); len(empty) > 0 {
		if hasFB {
			warns = append(warns,
				fmt.Sprintf("template has empty groups %s — left unchanged; filled only at runtime so start works",
					strings.Join(empty, ", ")))
		} else {
			warns = append(warns,
				fmt.Sprintf("template has empty groups %s and no direct/proxy fallback — add nodes via subscription before start",
					strings.Join(empty, ", ")))
		}
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
	name := "config." + base + ".json"
	if name == "config.config.json" {
		name = "config.imported.json"
	}
	dst := filepath.Join(dir, name)
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

	// Pretty-print original object only (no structural rewrites).
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
