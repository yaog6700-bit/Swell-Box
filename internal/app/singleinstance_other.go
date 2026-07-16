//go:build !windows

package app

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/swell-app/swellbox/internal/paths"
)

var singleInstanceFile *os.File

// AcquireSingleInstance locks ~/.swellbox/app.lock (flock).
func AcquireSingleInstance() (ok bool, err error) {
	dir, err := paths.HomeDir()
	if err != nil {
		return false, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return false, err
	}
	p := filepath.Join(dir, "app.lock")
	f, err := os.OpenFile(p, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return false, err
	}
	err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err != nil {
		_ = f.Close()
		return false, nil
	}
	// Write pid for debugging
	_, _ = f.Seek(0, 0)
	_ = f.Truncate(0)
	_, _ = fmt.Fprintf(f, "%d\n", os.Getpid())
	singleInstanceFile = f
	return true, nil
}

// ReleaseSingleInstance unlocks the lock file.
func ReleaseSingleInstance() {
	if singleInstanceFile == nil {
		return
	}
	_ = syscall.Flock(int(singleInstanceFile.Fd()), syscall.LOCK_UN)
	_ = singleInstanceFile.Close()
	singleInstanceFile = nil
}
