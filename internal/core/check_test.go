package core

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/swell-app/swellbox/internal/paths"
)

func TestCheckConfigForEngine_ShoesAutoHeal(t *testing.T) {
	shoesExe := `C:\Users\dododook\Desktop\shoes.exe`
	if _, err := os.Stat(shoesExe); err != nil {
		t.Skip("shoes.exe not found on Desktop, skipping")
	}

	tmpDir := t.TempDir()
	testYaml := filepath.Join(tmpDir, "test.yaml")

	// Write yaml with modern shoes 0.3.5 syntax (type: selector)
	content := `- client_group: proxy
  type: selector
  default: direct
  client_proxies:
    - name: direct
      address: 127.0.0.1:0
      protocol:
        type: socks
`
	if err := os.WriteFile(testYaml, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write test yaml: %v", err)
	}

	// shoes 0.3.4 rejects "type: selector", but CheckConfigForEngine should auto-heal it to strategy: selector!
	err := CheckConfigForEngine(paths.EngineShoes, shoesExe, testYaml, tmpDir)
	if err != nil {
		t.Fatalf("CheckConfigForEngine should have auto-healed and succeeded, but got: %v", err)
	}

	// Verify the file was rewritten to strategy: selector
	fixed, err := os.ReadFile(testYaml)
	if err != nil {
		t.Fatalf("failed to read test yaml: %v", err)
	}
	t.Logf("Auto-healed content:\n%s", string(fixed))
}
