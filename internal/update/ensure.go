package update

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/swell-app/swellbox/internal/paths"
)

// CorePresent reports whether the runtime core exists in ~/.swellbox/bin.
// The zip-side binary alone does not count — it must be seeded into the data dir.
func CorePresent() bool {
	name := paths.CoreBinaryName()
	if binDir, err := paths.BinDir(); err == nil {
		if st, err := os.Stat(filepath.Join(binDir, name)); err == nil && !st.IsDir() && st.Size() > 0 {
			return true
		}
	}
	return false
}

// EnsureCore makes sure ~/.swellbox/bin has a sing-box binary:
//  1. already in data dir
//  2. seed-copy from full.zip (next to Swell-Box.exe) into data dir
//  3. download from GitHub into data dir
//
// channel: stable or pre (used only for online download).
func EnsureCore(channel string, onProgress func(string)) (string, error) {
	if CorePresent() {
		return installedCoreVersion(), nil
	}

	// Seed from offline package next to this app (does not run from that path).
	if onProgress != nil {
		onProgress("bundle")
	}
	if ok, err := InstallBundledCore(); err == nil && ok && CorePresent() {
		return installedCoreVersion(), nil
	}

	// Online download into data dir
	if onProgress != nil {
		onProgress("downloading")
	}
	if channel == "" {
		channel = ChannelPre
	}
	ver, err := UpdateCore(channel, nil)
	if err != nil {
		if channel == ChannelPre {
			if onProgress != nil {
				onProgress("fallback-stable")
			}
			return UpdateCore(ChannelStable, nil)
		}
		return "", fmt.Errorf("no local core and download failed: %w", err)
	}
	return ver, nil
}

// EnsureCoreOrError is a one-liner for callers.
func EnsureCoreOrError(channel string) error {
	_, _ = InstallBundledCore()
	if CorePresent() {
		return nil
	}
	_, err := EnsureCore(channel, nil)
	if err != nil {
		return fmt.Errorf("core missing: place sing-box next to Swell-Box (seeds data dir), or connect network: %w", err)
	}
	return nil
}
