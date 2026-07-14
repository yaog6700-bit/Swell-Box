//go:build !windows

package app

import "os"

// IsElevated reports whether the process has privileges suitable for TUN.
// On Unix, root (uid 0) is treated as elevated; capabilities are not probed.
func IsElevated() bool {
	return os.Geteuid() == 0
}
