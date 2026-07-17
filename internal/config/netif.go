package config

import (
	"net"
	"strings"
)

// defaultOutboundInterface returns the OS interface name used for default
// egress (physical NIC). Used in TUN mode so "direct" does not re-enter the
// virtual adapter and loop. Empty string if detection fails.
func defaultOutboundInterface() string {
	if name := ifaceForUDPDial("udp4", "8.8.8.8:53"); name != "" {
		return name
	}
	return ifaceForUDPDial("udp6", "[2001:4860:4860::8888]:53")
}

func ifaceForUDPDial(network, address string) string {
	c, err := net.Dial(network, address)
	if err != nil {
		return ""
	}
	defer c.Close()
	la, ok := c.LocalAddr().(*net.UDPAddr)
	if !ok || la == nil || la.IP == nil {
		return ""
	}
	want := la.IP.To16()
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		if isVirtualOrTunnelName(iface.Name) {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			var ip net.IP
			switch v := a.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil {
				continue
			}
			if ip.To16() != nil && ip.Equal(want) {
				return iface.Name
			}
		}
	}
	return ""
}

func isVirtualOrTunnelName(name string) bool {
	n := strings.ToLower(name)
	for _, s := range []string{
		"wintun", "tun", "tap", "utun", "sing-box", "singbox", "swell",
		"loopback", "pseudo", "vethernet", "hyper-v", "vmware", "virtualbox",
		"docker", "wsl", "vpn",
	} {
		if strings.Contains(n, s) {
			return true
		}
	}
	return false
}
