//go:build !windows

package app

import "fmt"

// PickJSONFile is not implemented on non-Windows yet.
func PickJSONFile(title string) (string, error) {
	return "", fmt.Errorf("file dialog not supported on this OS yet")
}
