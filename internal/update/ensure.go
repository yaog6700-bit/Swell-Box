package update

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/swell-app/swellbox/internal/paths"
)

// CorePresent reports whether the runtime sing-box core exists in ~/.swellbox/bin.
func CorePresent() bool {
	return CorePresentFor(paths.EngineSingBox)
}

// CorePresentFor reports whether the specified runtime core exists in ~/.swellbox/bin.
func CorePresentFor(engine string) bool {
	name := paths.CoreBinaryNameFor(engine)
	if binDir, err := paths.BinDir(); err == nil {
		if st, err := os.Stat(filepath.Join(binDir, name)); err == nil && !st.IsDir() && st.Size() > 0 {
			return true
		}
	}
	return false
}

// EnsureCore makes sure ~/.swellbox/bin has a sing-box binary:
func EnsureCore(channel string, onProgress func(string)) (string, error) {
	return EnsureCoreForEngine(paths.EngineSingBox, channel, onProgress)
}

// EnsureCoreForEngine makes sure ~/.swellbox/bin has the specified core binary:
//  1. already in data dir
//  2. seed-copy from full.zip into data dir
//  3. download from GitHub into data dir
func EnsureCoreForEngine(engine, channel string, onProgress func(string)) (string, error) {
	if CorePresentFor(engine) {
		return InstalledCoreVersionFor(engine), nil
	}

	// Seed from offline package next to this app (does not run from that path).
	if onProgress != nil {
		onProgress("bundle")
	}
	if ok, err := InstallBundledCore(); err == nil && ok && CorePresentFor(engine) {
		return InstalledCoreVersionFor(engine), nil
	}

	// Online download into data dir
	if onProgress != nil {
		onProgress("downloading")
	}
	if channel == "" {
		channel = ChannelStable
	}
	ver, err := UpdateCoreForEngine(engine, channel, nil)
	if err != nil {
		if channel == ChannelPre {
			if onProgress != nil {
				onProgress("fallback-stable")
			}
			return UpdateCoreForEngine(engine, ChannelStable, nil)
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
