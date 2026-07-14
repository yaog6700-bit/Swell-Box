//go:build linux

package autostart

import (
	"fmt"
	"os"
	"path/filepath"
)

const desktopName = "Swell-Box.desktop"

func desktopPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	// XDG autostart
	return filepath.Join(home, ".config", "autostart", desktopName), nil
}

func Enabled() bool {
	p, err := desktopPath()
	if err != nil {
		return false
	}
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

func Enable() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	exe, err = filepath.Abs(exe)
	if err != nil {
		return err
	}
	if r, err := filepath.EvalSymlinks(exe); err == nil {
		exe = r
	}

	p, err := desktopPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}

	body := fmt.Sprintf(`[Desktop Entry]
Type=Application
Version=1.0
Name=Swell-Box
Comment=Swell-Box tray client for sing-box
Exec=%s
Terminal=false
Categories=Network;
X-GNOME-Autostart-enabled=true
`, exe)
	return os.WriteFile(p, []byte(body), 0o644)
}

func Disable() error {
	p, err := desktopPath()
	if err != nil {
		return err
	}
	_ = os.Remove(p)
	return nil
}

func Set(on bool) error {
	if on {
		return Enable()
	}
	return Disable()
}
