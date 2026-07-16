//go:build !windows

package core

import "os"

func assignToJob(p *os.Process) {}

// CloseJob is a no-op on non-Windows.
func CloseJob() {}
