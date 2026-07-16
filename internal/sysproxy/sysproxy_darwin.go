//go:build darwin

package sysproxy

import (
	"fmt"
	"os/exec"
	"strings"
	"sync"
)

var (
	mu           sync.Mutex
	weSetIt      bool
	saved        []serviceState
	lastHostPort string
)

type serviceState struct {
	name         string
	webOn        bool
	secureOn     bool
	socksOn      bool
	webHost      string
	webPort      string
	secureHost   string
	securePort   string
	socksHost    string
	socksPort    string
	bypassDomains string
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
			st.socksOn, st.socksHost, st.socksPort = readProxy("getsocksfirewallproxy", svc)
			st.bypassDomains = readBypass(svc)
			saved = append(saved, st)
		}
	}

	var lastErr error
	okAny := false
	for _, svc := range listServices() {
		if err := exec.Command("networksetup", "-setwebproxy", svc, host, port).Run(); err != nil {
			lastErr = err
			continue
		}
		if err := exec.Command("networksetup", "-setsecurewebproxy", svc, host, port).Run(); err != nil {
			lastErr = err
		}
		// mixed inbound also speaks SOCKS — many macOS apps ignore HTTP proxy only
		if err := exec.Command("networksetup", "-setsocksfirewallproxy", svc, host, port).Run(); err != nil {
			lastErr = err
		}
		_ = exec.Command("networksetup", "-setwebproxystate", svc, "on").Run()
		_ = exec.Command("networksetup", "-setsecurewebproxystate", svc, "on").Run()
		_ = exec.Command("networksetup", "-setsocksfirewallproxystate", svc, "on").Run()
		// Keep local / LAN off the proxy (same idea as Windows ProxyOverride)
		_ = exec.Command("networksetup", "-setproxybypassdomains", svc,
			"127.0.0.1", "localhost", "*.local", "169.254.0.0/16", "10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16",
		).Run()
		okAny = true
	}
	if !okAny && lastErr != nil {
		return lastErr
	}
	weSetIt = true
	lastHostPort = hostPort
	return nil
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
		// Still clear leftover localhost proxy if we own nothing in-process
		// (e.g. previous crash). Safe: only touches 127.0.0.1/localhost.
		return clearLocalhostLocked()
	}
	if len(saved) == 0 {
		weSetIt = false
		return disableAllServices()
	}
	for _, st := range saved {
		restoreOneProxy(st.name, "setwebproxy", "setwebproxystate", st.webOn, st.webHost, st.webPort)
		restoreOneProxy(st.name, "setsecurewebproxy", "setsecurewebproxystate", st.secureOn, st.secureHost, st.securePort)
		restoreOneProxy(st.name, "setsocksfirewallproxy", "setsocksfirewallproxystate", st.socksOn, st.socksHost, st.socksPort)
		if st.bypassDomains != "" {
			// networksetup wants space-separated list as separate args when possible;
			// empty means restore defaults — skip if we never captured useful data.
			parts := strings.Fields(st.bypassDomains)
			if len(parts) > 0 {
				args := append([]string{"-setproxybypassdomains", st.name}, parts...)
				_ = exec.Command("networksetup", args...).Run()
			}
		}
	}
	weSetIt = false
	saved = nil
	lastHostPort = ""
	return clearLocalhostLocked()
}

func restoreOneProxy(svc, setCmd, stateCmd string, on bool, host, port string) {
	if on && host != "" && port != "" && !isOurHostPort(host, port) {
		_ = exec.Command("networksetup", "-"+setCmd, svc, host, port).Run()
		_ = exec.Command("networksetup", "-"+stateCmd, svc, "on").Run()
		return
	}
	_ = exec.Command("networksetup", "-"+stateCmd, svc, "off").Run()
}

func disableAllServices() error {
	var lastErr error
	for _, svc := range listServices() {
		if err := exec.Command("networksetup", "-setwebproxystate", svc, "off").Run(); err != nil {
			lastErr = err
		}
		_ = exec.Command("networksetup", "-setsecurewebproxystate", svc, "off").Run()
		_ = exec.Command("networksetup", "-setsocksfirewallproxystate", svc, "off").Run()
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

func readBypass(svc string) string {
	out, err := exec.Command("networksetup", "-getproxybypassdomains", svc).Output()
	if err != nil {
		return ""
	}
	s := strings.TrimSpace(string(out))
	if strings.Contains(strings.ToLower(s), "there aren't any") {
		return ""
	}
	// multi-line list → space separated
	var parts []string
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			parts = append(parts, line)
		}
	}
	return strings.Join(parts, " ")
}

func splitHostPort(s string) (host, port string, ok bool) {
	// host:port — IPv6 not expected for local mixed inbound
	i := strings.LastIndex(s, ":")
	if i <= 0 || i == len(s)-1 {
		return "", "", false
	}
	return s[:i], s[i+1:], true
}

func isOurHostPort(host, port string) bool {
	h := strings.ToLower(strings.TrimSpace(host))
	p := strings.TrimSpace(port)
	if h != "127.0.0.1" && h != "localhost" {
		return false
	}
	if p == "7890" || p == "7080" {
		return true
	}
	if lastHostPort != "" {
		_, lp, ok := splitHostPort(lastHostPort)
		if ok && lp == p {
			return true
		}
	}
	// Any localhost proxy is almost certainly a leftover local client.
	return true
}

// ClearLeftover disables system proxy if it still points at a local Swell-Box
// port after a crash / unclean exit. Safe to call on every app start.
func ClearLeftover() error {
	mu.Lock()
	defer mu.Unlock()
	return clearLocalhostLocked()
}

func clearLocalhostLocked() error {
	var lastErr error
	for _, svc := range listServices() {
		webOn, webHost, webPort := readProxy("getwebproxy", svc)
		secureOn, secureHost, securePort := readProxy("getsecurewebproxy", svc)
		socksOn, socksHost, socksPort := readProxy("getsocksfirewallproxy", svc)

		if webOn && isOurHostPort(webHost, webPort) {
			if err := exec.Command("networksetup", "-setwebproxystate", svc, "off").Run(); err != nil {
				lastErr = err
			}
		}
		if secureOn && isOurHostPort(secureHost, securePort) {
			_ = exec.Command("networksetup", "-setsecurewebproxystate", svc, "off").Run()
		}
		if socksOn && isOurHostPort(socksHost, socksPort) {
			_ = exec.Command("networksetup", "-setsocksfirewallproxystate", svc, "off").Run()
		}
	}
	weSetIt = false
	return lastErr
}
