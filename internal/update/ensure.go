package update

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/swell-app/swellbox/internal/paths"
)

// CorePresent reports whether a sing-box binary is already available.
func CorePresent() bool {
	name := paths.CoreBinaryName()
	if binDir, err := paths.BinDir(); err == nil {
		if st, err := os.Stat(filepath.Join(binDir, name)); err == nil && !st.IsDir() {
			return true
		}
	}
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		for _, p := range []string{
			filepath.Join(dir, name),
			filepath.Join(dir, "bin", name),
		} {
			if st, err := os.Stat(p); err == nil && !st.IsDir() {
				return true
			}
		}
	}
	return false
}

// EnsureCore makes sure a core binary is available:
//  1. already in ~/.swellbox/bin or next to exe
//  2. copy from package (next to SWELL-Box.exe) into data dir
//  3. download from GitHub (needs network)
//
// channel: stable or pre (used only for online download).
func EnsureCore(channel string, onProgress func(string)) (string, error) {
	// Prefer data-dir install for long-term use.
	if binDir, err := paths.BinDir(); err == nil {
		p := filepath.Join(binDir, paths.CoreBinaryName())
		if st, err := os.Stat(p); err == nil && !st.IsDir() && st.Size() > 0 {
			return installedCoreVersion(), nil
		}
	}

	// Install from release package sitting next to this app (offline-friendly).
	if onProgress != nil {
		onProgress("bundle")
	}
	if ok, err := InstallBundledCore(); err == nil && ok {
		return installedCoreVersion(), nil
	} else if CorePresent() {
		// May still be next to exe without copy
		return installedCoreVersion(), nil
	}

	// Online download fallback
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
	if CorePresent() {
		// still try bundle install into data dir if only next-to-exe
		_, _ = InstallBundledCore()
		return nil
	}
	_, err := EnsureCore(channel, nil)
	if err != nil {
		return fmt.Errorf("core missing: place sing-box.exe next to SWELL-Box.exe, or connect network for auto-download: %w", err)
	}
	return nil
}
