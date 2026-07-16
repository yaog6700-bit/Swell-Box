//go:build linux

package sysproxy

import (
	"fmt"
	"os/exec"
	"strings"
	"sync"
)

var (
	mu      sync.Mutex
	weSetIt bool
	oldMode string
)

func hasGsettings() bool {
	_, err := exec.LookPath("gsettings")
	return err == nil
}

func IsOn() bool {
	if !hasGsettings() {
		return false
	}
	out, err := exec.Command("gsettings", "get", "org.gnome.system.proxy", "mode").Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), "manual")
}

func Enable(hostPort string) error {
	mu.Lock()
	defer mu.Unlock()

	host, port, ok := splitHostPort(hostPort)
	if !ok {
		return fmt.Errorf("invalid proxy address: %s", hostPort)
	}
	if !hasGsettings() {
		return fmt.Errorf("system proxy: gsettings not available (GNOME); set browser/app proxy to %s manually", hostPort)
	}

	if !weSetIt {
		if out, err := exec.Command("gsettings", "get", "org.gnome.system.proxy", "mode").Output(); err == nil {
			oldMode = strings.Trim(strings.TrimSpace(string(out)), "'")
		}
	}

	// HTTP + HTTPS manual proxy (mixed inbound accepts both)
	_ = exec.Command("gsettings", "set", "org.gnome.system.proxy.http", "host", host).Run()
	_ = exec.Command("gsettings", "set", "org.gnome.system.proxy.http", "port", port).Run()
	_ = exec.Command("gsettings", "set", "org.gnome.system.proxy.https", "host", host).Run()
	_ = exec.Command("gsettings", "set", "org.gnome.system.proxy.https", "port", port).Run()
	if err := exec.Command("gsettings", "set", "org.gnome.system.proxy", "mode", "manual").Run(); err != nil {
		return err
	}
	weSetIt = true
	return nil
}

func Disable() error {
	mu.Lock()
	defer mu.Unlock()
	if !hasGsettings() {
		weSetIt = false
		return nil
	}
	err := exec.Command("gsettings", "set", "org.gnome.system.proxy", "mode", "none").Run()
	weSetIt = false
	return err
}

func Restore() error {
	mu.Lock()
	defer mu.Unlock()
	if !weSetIt {
		return nil
	}
	if !hasGsettings() {
		weSetIt = false
		return nil
	}
	mode := oldMode
	if mode == "" {
		mode = "none"
	}
	err := exec.Command("gsettings", "set", "org.gnome.system.proxy", "mode", mode).Run()
	weSetIt = false
	oldMode = ""
	return err
}

func WeOwn() bool {
	mu.Lock()
	defer mu.Unlock()
	return weSetIt
}

func splitHostPort(s string) (host, port string, ok bool) {
	i := strings.LastIndex(s, ":")
	if i <= 0 || i == len(s)-1 {
		return "", "", false
	}
	return s[:i], s[i+1:], true
}

// ClearLeftover is a no-op on Linux (handled by Restore).
func ClearLeftover() error { return nil }

