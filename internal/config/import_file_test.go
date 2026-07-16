package config

import "testing"

func TestSanitizeEmptyURLTest(t *testing.T) {
	root := map[string]any{
		"outbounds": []any{
			map[string]any{"type": "direct", "tag": "Direct"},
			map[string]any{
				"type":      "urltest",
				"tag":       "HongKong",
				"outbounds": []any{},
			},
			map[string]any{
				"type":        "shadowsocks",
				"tag":         "SG",
				"server":      "1.2.3.4",
				"server_port": 443,
				"method":      "aes-256-gcm",
				"password":    "x",
			},
		},
	}
	warns, err := sanitizeOutboundGroups(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(warns) != 1 {
		t.Fatalf("warns=%v", warns)
	}
	outbounds := root["outbounds"].([]any)
	hk := outbounds[1].(map[string]any)
	list := toStringSlice(hk["outbounds"])
	if len(list) != 1 || list[0] != "Direct" {
		t.Fatalf("filled=%v want Direct", list)
	}
}

func TestSanitizeEmptyNoFallback(t *testing.T) {
	root := map[string]any{
		"outbounds": []any{
			map[string]any{
				"type":      "urltest",
				"tag":       "Empty",
				"outbounds": []any{},
			},
		},
	}
	_, err := sanitizeOutboundGroups(root)
	if err == nil {
		t.Fatal("expected error")
	}
}
