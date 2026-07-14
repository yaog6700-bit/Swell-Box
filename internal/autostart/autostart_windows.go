//go:build windows

package autostart

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows/registry"
)

const regName = "Swell-Box"

func regKey() (registry.Key, error) {
	return registry.OpenKey(
		registry.CURRENT_USER,
		`Software\Microsoft\Windows\CurrentVersion\Run`,
		registry.QUERY_VALUE|registry.SET_VALUE,
	)
}

// Enabled reports whether login autostart is registered.
func Enabled() bool {
	k, err := regKey()
	if err != nil {
		return false
	}
	defer k.Close()
	_, _, err = k.GetStringValue(regName)
	return err == nil
}

// Enable registers the current executable to run at user login.
func Enable() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	exe, err = filepath.Abs(exe)
	if err != nil {
		return err
	}
	// Quote path for spaces.
	val := `"` + exe + `"`
	k, err := regKey()
	if err != nil {
		return err
	}
	defer k.Close()
	return k.SetStringValue(regName, val)
}

// Disable removes login autostart.
func Disable() error {
	k, err := regKey()
	if err != nil {
		return err
	}
	defer k.Close()
	_ = k.DeleteValue(regName)
	return nil
}

// Set enables or disables autostart.
func Set(on bool) error {
	if on {
		return Enable()
	}
	if err := Disable(); err != nil {
		return fmt.Errorf("disable autostart: %w", err)
	}
	return nil
}
