package config

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestPrepareRuntimeShoesConfig(t *testing.T) {
	// Test translating user_config.json if it exists in workspace
	workspaceConfig := filepath.Join("..", "..", "user_config.json")
	if _, err := os.Stat(workspaceConfig); err != nil {
		t.Skip("user_config.json not found, skipping")
	}

	tmpDir := t.TempDir()
	outYaml := filepath.Join(tmpDir, "runtime.shoes.yaml")

	err := PrepareRuntimeShoesConfig(workspaceConfig, outYaml, "", 9090, 7890, "s3cret")
	if err != nil {
		t.Fatalf("PrepareRuntimeShoesConfig failed: %v", err)
	}

	data, err := os.ReadFile(outYaml)
	if err != nil {
		t.Fatalf("Failed to read generated yaml: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("Generated yaml is empty")
	}

	// If shoes.exe exists on Desktop, run check on it
	shoesExe := `C:\Users\dododook\Desktop\shoes.exe`
	if _, err := os.Stat(shoesExe); err == nil {
		cmd := exec.Command(shoesExe, "check", outYaml)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("shoes check failed: %v, output: %s", err, string(out))
		}
	}
}

func TestTranslateShadowsocks2022MultiUser(t *testing.T) {
	ob := map[string]any{
		"type":        "shadowsocks",
		"method":      "2022-blake3-aes-256-gcm",
		"server":      "1.2.3.4",
		"server_port": 8388,
		"password":    "+1DCOG5hPikjoizvd6dNJezy9YAiOVCo32+zWl9BJsI=:sjB15DAjU2qtctI+AGUuFztyXeXkBlhxW+0tjWsSedc=",
	}

	res := translateOutboundToShoes("SS2022-MultiUser", ob)
	if res == nil {
		t.Fatal("expected SS2022 multi-user node to be translated, but got nil")
	}

	proto, ok := res["protocol"].(map[string]any)
	if !ok {
		t.Fatalf("expected protocol map, got: %v", res["protocol"])
	}
	if proto["type"] != "shadowsocks" || proto["cipher"] != "2022-blake3-aes-256-gcm" {
		t.Fatalf("unexpected protocol fields: %v", proto)
	}
	if proto["password"] != ob["password"] {
		t.Fatalf("password mismatch: got %v, want %v", proto["password"], ob["password"])
	}
}
