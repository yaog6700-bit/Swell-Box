package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestListSwitchableNodesOnlyDefaultConfigJSON(t *testing.T) {
	dir := t.TempDir()

	// Even with proxy group, non-default filename → empty.
	pathOther := filepath.Join(dir, "config.sing-box.json")
	bodyProxy := `{
  "outbounds": [
    {"tag":"proxy","type":"selector","outbounds":["SG-FREE","direct"],"default":"SG-FREE"},
    {"tag":"direct","type":"direct"},
    {"tag":"SG-FREE","type":"shadowsocks","server":"1.2.3.4","server_port":1,"method":"aes-256-gcm","password":"x"}
  ]
}`
	if err := os.WriteFile(pathOther, []byte(bodyProxy), 0o644); err != nil {
		t.Fatal(err)
	}
	primary, nodes, _, err := ListSwitchableNodes(pathOther)
	if err != nil {
		t.Fatal(err)
	}
	if primary != "" || len(nodes) != 0 {
		t.Fatalf("non-default name should be empty, got primary=%q nodes=%+v", primary, nodes)
	}

	// Default config.json + proxy leaves → show.
	pathHome := filepath.Join(dir, "config.json")
	if err := os.WriteFile(pathHome, []byte(bodyProxy), 0o644); err != nil {
		t.Fatal(err)
	}
	primary, nodes, cur, err := ListSwitchableNodes(pathHome)
	if err != nil {
		t.Fatal(err)
	}
	if primary != "proxy" || len(nodes) != 1 || nodes[0].Tag != "SG-FREE" {
		t.Fatalf("primary=%s nodes=%+v cur=%s", primary, nodes, cur)
	}
}
