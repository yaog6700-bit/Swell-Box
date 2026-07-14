//go:build !windows && !darwin && !linux

package app

import "fmt"

// PickJSONFile is not implemented on this OS.
func PickJSONFile(title string) (string, error) {
	return "", fmt.Errorf("file dialog not supported on this OS")
}
