//go:build !windows

package update

import "fmt"

// ApplyAppUpdate is only implemented on Windows for now.
// macOS (.app) / Linux need platform-specific install paths.
func ApplyAppUpdate(downloadURL string, isZip bool, stopFn func() error) error {
	return fmt.Errorf("in-app update is only supported on Windows currently; please download the release package manually")
}
