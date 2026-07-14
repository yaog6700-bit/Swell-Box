package core

import (
	"io"
	"os"
)

const (
	// maxLogBytes: rotate when core.log exceeds this size (~5 MiB).
	maxLogBytes = 5 << 20
	// keepLogBytes: after rotate, keep only the last N bytes.
	keepLogBytes = 1 << 20
)

// rotateLogIfNeeded trims core.log when it grows too large.
// Strategy: keep the last keepBytes, move old file to core.log.1 (one backup).
func rotateLogIfNeeded(path string, maxBytes, keepBytes int64) {
	st, err := os.Stat(path)
	if err != nil || st.Size() < maxBytes {
		return
	}
	// Backup previous (overwrite single .1)
	_ = os.Remove(path + ".1")
	_ = os.Rename(path, path+".1")

	// Optionally leave a truncated tail as new core.log for continuity
	if keepBytes <= 0 {
		return
	}
	old, err := os.Open(path + ".1")
	if err != nil {
		return
	}
	defer old.Close()
	size := st.Size()
	start := size - keepBytes
	if start < 0 {
		start = 0
	}
	if _, err := old.Seek(start, io.SeekStart); err != nil {
		return
	}
	out, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return
	}
	defer out.Close()
	_, _ = io.Copy(out, old)
}
