//go:build !windows && !darwin && !linux

package sysproxy

func IsOn() bool                   { return false }
func Enable(hostPort string) error { return nil }
func Disable() error               { return nil }
func Restore() error               { return nil }
func ClearLeftover() error         { return nil }
func WeOwn() bool                  { return false }
