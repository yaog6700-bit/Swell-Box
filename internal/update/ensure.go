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
		if st, err := os.Stat(filepath.Join(filepath.Dir(exe), name)); err == nil && !st.IsDir() {
			return true
		}
	}
	return false
}

// EnsureCore downloads core if missing. channel: stable or pre.
// onProgress is optional status callback (e.g. for notifications).
func EnsureCore(channel string, onProgress func(string)) (string, error) {
	if CorePresent() {
		return installedCoreVersion(), nil
	}
	if onProgress != nil {
		onProgress("downloading")
	}
	if channel == "" {
		// Dashboard API needs sing-box 1.14+; prefer pre until 1.14 is stable.
		channel = ChannelPre
	}
	ver, err := UpdateCore(channel, nil)
	if err != nil {
		// fallback stable once
		if channel == ChannelPre {
			if onProgress != nil {
				onProgress("fallback-stable")
			}
			return UpdateCore(ChannelStable, nil)
		}
		return "", err
	}
	return ver, nil
}

// EnsureCoreOrError is a one-liner for callers.
func EnsureCoreOrError(channel string) error {
	if CorePresent() {
		return nil
	}
	_, err := EnsureCore(channel, nil)
	if err != nil {
		return fmt.Errorf("auto download core: %w", err)
	}
	return nil
}
