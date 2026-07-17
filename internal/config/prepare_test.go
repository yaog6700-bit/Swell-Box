package config

import "testing"

func TestApplyTunModeInjectsDualStack(t *testing.T) {
	root := map[string]any{
		"inbounds": []any{
			map[string]any{"type": "mixed", "tag": "mixed-in", "listen": "127.0.0.1", "listen_port": 7890},
		},
	}
	applyTunMode(root, true)

	inbounds, _ := root["inbounds"].([]any)
	var tun map[string]any
	for _, item := range inbounds {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if tag, _ := m["tag"].(string); tag == SwellTunTag {
			tun = m
			break
		}
	}
	if tun == nil {
		t.Fatal("swell-tun inbound not injected")
	}
	addrs, _ := tun["address"].([]any)
	if len(addrs) < 2 {
		t.Fatalf("address=%v want IPv4+IPv6", addrs)
	}
	var hasV4, hasV6 bool
	for _, a := range addrs {
		s, _ := a.(string)
		if s == "172.19.0.1/30" {
			hasV4 = true
		}
		if s == "fdfe:dcba:9876::1/126" {
			hasV6 = true
		}
	}
	if !hasV4 || !hasV6 {
		t.Fatalf("address=%v missing dual-stack entries", addrs)
	}
	if tun["auto_route"] != true {
		t.Fatalf("auto_route not set: %#v", tun)
	}
	// Must match Anywhere: strict_route false so IPv6/WFP is not blackholed.
	if tun["strict_route"] != false {
		t.Fatalf("strict_route=%v want false", tun["strict_route"])
	}
	ex, _ := tun["route_exclude_address"].([]any)
	if len(ex) < 4 {
		t.Fatalf("route_exclude_address too short: %v", ex)
	}
	route, _ := root["route"].(map[string]any)
	if route["auto_detect_interface"] != true {
		t.Fatalf("route.auto_detect_interface=%v", route["auto_detect_interface"])
	}
}

func TestBindDirectOutbounds(t *testing.T) {
	root := map[string]any{
		"outbounds": []any{
			map[string]any{"type": "selector", "tag": "proxy", "outbounds": []any{"direct"}},
			map[string]any{"type": "direct", "tag": "direct"},
			map[string]any{"type": "direct", "tag": "direct-bound", "bind_interface": "keep-me"},
		},
	}
	bindDirectOutbounds(root, "Ethernet")
	outs := root["outbounds"].([]any)
	d := outs[1].(map[string]any)
	if d["bind_interface"] != "Ethernet" {
		t.Fatalf("direct bind=%v", d["bind_interface"])
	}
	kept := outs[2].(map[string]any)
	if kept["bind_interface"] != "keep-me" {
		t.Fatalf("should not override existing bind: %v", kept["bind_interface"])
	}
}

func TestApplyTunModeDisabledStripsTun(t *testing.T) {
	root := map[string]any{
		"inbounds": []any{
			map[string]any{"type": "tun", "tag": "user-tun", "address": []any{"172.19.0.1/30"}},
			map[string]any{"type": "mixed", "tag": "mixed-in"},
		},
	}
	applyTunMode(root, false)
	for _, item := range root["inbounds"].([]any) {
		m := item.(map[string]any)
		if tpe, _ := m["type"].(string); tpe == "tun" {
			t.Fatalf("tun should be stripped when disabled: %#v", m)
		}
	}
}

func TestApplyTunModeKeepsUserTun(t *testing.T) {
	root := map[string]any{
		"inbounds": []any{
			map[string]any{"type": "tun", "tag": "user-tun", "address": []any{"10.0.0.1/30"}},
		},
	}
	applyTunMode(root, true)
	inbounds := root["inbounds"].([]any)
	if len(inbounds) != 1 {
		t.Fatalf("len=%d want 1 (user tun kept, no double inject)", len(inbounds))
	}
	m := inbounds[0].(map[string]any)
	if m["tag"] != "user-tun" {
		t.Fatalf("tag=%v", m["tag"])
	}
}
