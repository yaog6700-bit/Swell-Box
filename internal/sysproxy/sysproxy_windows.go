//go:build windows

package sysproxy

import (
	"strings"
	"sync"
	"syscall"

	"golang.org/x/sys/windows/registry"
)

const regPath = `Software\Microsoft\Windows\CurrentVersion\Internet Settings`

var (
	mu          sync.Mutex
	saved       bool
	oldEnable   uint64
	oldServer   string
	oldOverride string
	weSetIt     bool
	lastHostPort string
)

// IsOn reports whether system proxy is currently enabled.
func IsOn() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, regPath, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	v, _, err := k.GetIntegerValue("ProxyEnable")
	return err == nil && v == 1
}

// Enable sets system proxy to host:port (e.g. 127.0.0.1:7890).
func Enable(hostPort string) error {
	mu.Lock()
	defer mu.Unlock()

	k, err := registry.OpenKey(registry.CURRENT_USER, regPath, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()

	if !saved {
		if v, _, err := k.GetIntegerValue("ProxyEnable"); err == nil {
			oldEnable = v
		}
		if s, _, err := k.GetStringValue("ProxyServer"); err == nil {
			oldServer = s
		}
		if s, _, err := k.GetStringValue("ProxyOverride"); err == nil {
			oldOverride = s
		}
		saved = true
	}

	if err := k.SetStringValue("ProxyServer", hostPort); err != nil {
		return err
	}
	_ = k.SetStringValue("ProxyOverride", "<local>;localhost;127.*;10.*;192.168.*;*.local")
	if err := k.SetDWordValue("ProxyEnable", 1); err != nil {
		return err
	}
	weSetIt = true
	lastHostPort = hostPort
	refresh()
	return nil
}

// Disable turns system proxy off (ProxyEnable=0).
func Disable() error {
	mu.Lock()
	defer mu.Unlock()
	return disableLocked()
}

func disableLocked() error {
	k, err := registry.OpenKey(registry.CURRENT_USER, regPath, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	if err := k.SetDWordValue("ProxyEnable", 0); err != nil {
		return err
	}
	weSetIt = false
	refresh()
	return nil
}

// Restore puts back settings captured before we enabled proxy.
// Always ensures ProxyEnable is off when we had turned it on, so stopping
// the core cannot leave Windows stuck on a dead 127.0.0.1 proxy.
func Restore() error {
	mu.Lock()
	defer mu.Unlock()

	// If we never touched settings, still clear leftover localhost proxy from a crash.
	if !saved && !weSetIt {
		return clearLocalhostLocked()
	}

	k, err := registry.OpenKey(registry.CURRENT_USER, regPath, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()

	if saved {
		// Prefer restoring pre-Swell state, but never leave Enable=1 pointing at us.
		wantEnable := uint32(oldEnable)
		if weSetIt {
			// We own the session: turn off unless user previously had another proxy.
			if isOurServer(oldServer) || oldServer == "" || oldServer == lastHostPort {
				wantEnable = 0
			}
		}
		_ = k.SetDWordValue("ProxyEnable", wantEnable)
		if oldServer != "" && !isOurServer(oldServer) {
			_ = k.SetStringValue("ProxyServer", oldServer)
		} else if wantEnable == 0 {
			// Leave ProxyServer string; Enable=0 is what matters for browsing.
		}
		if oldOverride != "" {
			_ = k.SetStringValue("ProxyOverride", oldOverride)
		}
	} else {
		_ = k.SetDWordValue("ProxyEnable", 0)
	}
	weSetIt = false
	refresh()
	// Belt-and-suspenders: if still enabled to localhost, force off.
	return clearLocalhostLocked()
}

// ClearLeftover disables system proxy if it still points at a local Swell-Box
// port after a crash / unclean exit. Safe to call on every app start.
func ClearLeftover() error {
	mu.Lock()
	defer mu.Unlock()
	return clearLocalhostLocked()
}

func clearLocalhostLocked() error {
	k, err := registry.OpenKey(registry.CURRENT_USER, regPath, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	en, _, err := k.GetIntegerValue("ProxyEnable")
	if err != nil || en == 0 {
		return nil
	}
	server, _, err := k.GetStringValue("ProxyServer")
	if err != nil {
		return nil
	}
	if !isOurServer(server) {
		return nil
	}
	if err := k.SetDWordValue("ProxyEnable", 0); err != nil {
		return err
	}
	weSetIt = false
	refresh()
	return nil
}

func isOurServer(server string) bool {
	s := strings.ToLower(strings.TrimSpace(server))
	if s == "" {
		return false
	}
	// Common mixed-inbound ports we set (default 7890; imported templates may use 7080).
	if strings.Contains(s, "127.0.0.1:7890") || strings.Contains(s, "localhost:7890") {
		return true
	}
	if strings.Contains(s, "127.0.0.1:7080") || strings.Contains(s, "localhost:7080") {
		return true
	}
	if lastHostPort != "" && strings.Contains(s, strings.ToLower(lastHostPort)) {
		return true
	}
	// Generic: any 127.0.0.1 proxy is almost certainly a local client we left behind.
	if strings.HasPrefix(s, "127.0.0.1:") || strings.HasPrefix(s, "localhost:") {
		return true
	}
	return false
}

// WeOwn reports whether Swell-Box currently enabled the proxy.
func WeOwn() bool {
	mu.Lock()
	defer mu.Unlock()
	return weSetIt
}

func refresh() {
	wininet := syscall.NewLazyDLL("wininet.dll")
	proc := wininet.NewProc("InternetSetOptionW")
	// INTERNET_OPTION_SETTINGS_CHANGED = 39, INTERNET_OPTION_REFRESH = 37
	_, _, _ = proc.Call(0, 39, 0, 0)
	_, _, _ = proc.Call(0, 37, 0, 0)
}
