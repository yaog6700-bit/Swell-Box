//go:build windows

package sysproxy

import (
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
	refresh()
	return nil
}

// Disable turns system proxy off.
func Disable() error {
	mu.Lock()
	defer mu.Unlock()
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
func Restore() error {
	mu.Lock()
	defer mu.Unlock()
	if !saved && !weSetIt {
		return nil
	}
	k, err := registry.OpenKey(registry.CURRENT_USER, regPath, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()

	if saved {
		_ = k.SetDWordValue("ProxyEnable", uint32(oldEnable))
		if oldServer != "" {
			_ = k.SetStringValue("ProxyServer", oldServer)
		}
		if oldOverride != "" {
			_ = k.SetStringValue("ProxyOverride", oldOverride)
		}
	} else {
		_ = k.SetDWordValue("ProxyEnable", 0)
	}
	weSetIt = false
	refresh()
	return nil
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
