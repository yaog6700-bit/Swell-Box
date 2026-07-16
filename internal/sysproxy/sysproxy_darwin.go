//go:build darwin

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
	saved   []serviceState
)

type serviceState struct {
	name       string
	webOn      bool
	secureOn   bool
	webHost    string
	webPort    string
	secureHost string
	securePort string
}

// network services that commonly carry user traffic
func listServices() []string {
	out, err := exec.Command("networksetup", "-listallnetworkservices").Output()
	if err != nil {
		return []string{"Wi-Fi", "Ethernet"}
	}
	var svcs []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "An asterisk") {
			continue
		}
		// disabled services are prefixed with *
		if strings.HasPrefix(line, "*") {
			continue
		}
		svcs = append(svcs, line)
	}
	if len(svcs) == 0 {
		return []string{"Wi-Fi"}
	}
	return svcs
}

func IsOn() bool {
	for _, svc := range listServices() {
		out, err := exec.Command("networksetup", "-getwebproxy", svc).Output()
		if err != nil {
			continue
		}
		if strings.Contains(string(out), "Enabled: Yes") {
			return true
		}
	}
	return false
}

func Enable(hostPort string) error {
	mu.Lock()
	defer mu.Unlock()

	host, port, ok := splitHostPort(hostPort)
	if !ok {
		return fmt.Errorf("invalid proxy address: %s", hostPort)
	}

	if !weSetIt {
		saved = nil
		for _, svc := range listServices() {
			st := serviceState{name: svc}
			st.webOn, st.webHost, st.webPort = readProxy("getwebproxy", svc)
			st.secureOn, st.secureHost, st.securePort = readProxy("getsecurewebproxy", svc)
			saved = append(saved, st)
		}
	}

	var lastErr error
	for _, svc := range listServices() {
		if err := exec.Command("networksetup", "-setwebproxy", svc, host, port).Run(); err != nil {
			lastErr = err
			continue
		}
		if err := exec.Command("networksetup", "-setsecurewebproxy", svc, host, port).Run(); err != nil {
			lastErr = err
		}
		_ = exec.Command("networksetup", "-setwebproxystate", svc, "on").Run()
		_ = exec.Command("networksetup", "-setsecurewebproxystate", svc, "on").Run()
	}
	weSetIt = true
	return lastErr
}

func Disable() error {
	mu.Lock()
	defer mu.Unlock()
	weSetIt = false
	return disableAllServices()
}

func Restore() error {
	mu.Lock()
	defer mu.Unlock()
	if !weSetIt {
		return nil
	}
	if len(saved) == 0 {
		weSetIt = false
		return disableAllServices()
	}
	for _, st := range saved {
		if st.webOn && st.webHost != "" {
			_ = exec.Command("networksetup", "-setwebproxy", st.name, st.webHost, st.webPort).Run()
			_ = exec.Command("networksetup", "-setwebproxystate", st.name, "on").Run()
		} else {
			_ = exec.Command("networksetup", "-setwebproxystate", st.name, "off").Run()
		}
		if st.secureOn && st.secureHost != "" {
			_ = exec.Command("networksetup", "-setsecurewebproxy", st.name, st.secureHost, st.securePort).Run()
			_ = exec.Command("networksetup", "-setsecurewebproxystate", st.name, "on").Run()
		} else {
			_ = exec.Command("networksetup", "-setsecurewebproxystate", st.name, "off").Run()
		}
	}
	weSetIt = false
	saved = nil
	return nil
}

func disableAllServices() error {
	var lastErr error
	for _, svc := range listServices() {
		if err := exec.Command("networksetup", "-setwebproxystate", svc, "off").Run(); err != nil {
			lastErr = err
		}
		_ = exec.Command("networksetup", "-setsecurewebproxystate", svc, "off").Run()
	}
	return lastErr
}

func WeOwn() bool {
	mu.Lock()
	defer mu.Unlock()
	return weSetIt
}

func readProxy(cmd, svc string) (on bool, host, port string) {
	out, err := exec.Command("networksetup", "-"+cmd, svc).Output()
	if err != nil {
		return false, "", ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "Enabled:"):
			on = strings.Contains(line, "Yes")
		case strings.HasPrefix(line, "Server:"):
			host = strings.TrimSpace(strings.TrimPrefix(line, "Server:"))
		case strings.HasPrefix(line, "Port:"):
			port = strings.TrimSpace(strings.TrimPrefix(line, "Port:"))
		}
	}
	return on, host, port
}

func splitHostPort(s string) (host, port string, ok bool) {
	// host:port — IPv6 not expected for local mixed inbound
	i := strings.LastIndex(s, ":")
	if i <= 0 || i == len(s)-1 {
		return "", "", false
	}
	return s[:i], s[i+1:], true
}

// ClearLeftover is a no-op on Darwin (handled by Restore).
func ClearLeftover() error { return nil }

