//go:build !windows && !darwin

package update

import "fmt"

// ApplyAppUpdate is only implemented on Windows and macOS for now.
func ApplyAppUpdate(downloadURL string, isZip bool, stopFn func() error) error {
	return fmt.Errorf("in-app update is not supported on this platform; please download the release package manually")
}
