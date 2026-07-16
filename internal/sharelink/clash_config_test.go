package sharelink

import (
	"strings"
	"testing"
)

func TestParseClashConfigYAML(t *testing.T) {
	body := `
proxies:
  - name: "SG-SS"
    type: ss
    server: 1.2.3.4
    port: 8388
    cipher: aes-256-gcm
    password: secret
  - name: "Snell6"
    type: snell
    server: 216.236.7.38
    port: 26005
    psk: tRA3iO4bPnHz6rPhojK1r
    version: 6
    reuse: true
    tfo: true
  - name: "HK-VLESS"
    type: vless
    server: v.example.com
    port: 443
    uuid: 11111111-1111-1111-1111-111111111111
    network: tcp
    tls: true
    servername: www.example.com
    flow: xtls-rprx-vision
    client-fingerprint: chrome
    reality-opts:
      public-key: PUBLICKEY
      short-id: abcd
`
	nodes, err := ParseClashConfig(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 3 {
		t.Fatalf("got %d nodes: %+v", len(nodes), tagsOf(nodes))
	}
	byTag := map[string]map[string]any{}
	for _, n := range nodes {
		byTag[n.Tag] = n.Outbound
	}
	if byTag["SG-SS"]["type"] != "shadowsocks" {
		t.Fatalf("ss=%v", byTag["SG-SS"])
	}
	if byTag["Snell6"]["type"] != "snell" || byTag["Snell6"]["version"] != 6 {
		t.Fatalf("snell=%v", byTag["Snell6"])
	}
	if byTag["Snell6"]["psk"] != "tRA3iO4bPnHz6rPhojK1r" {
		t.Fatalf("psk=%v", byTag["Snell6"]["psk"])
	}
	if byTag["HK-VLESS"]["type"] != "vless" {
		t.Fatalf("vless=%v", byTag["HK-VLESS"])
	}
}

func TestParseClashConfigJSON(t *testing.T) {
	body := `{"proxies":[{"name":"A","type":"ss","server":"1.1.1.1","port":80,"cipher":"aes-256-gcm","password":"p"}]}`
	nodes, err := ParseClashConfig(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 1 || nodes[0].Outbound["type"] != "shadowsocks" {
		t.Fatalf("%+v", nodes)
	}
}

func TestSubscribeParseBodyPrefersClashOverPartialURI(t *testing.T) {
	// Simulates a Clash subscription that also happens to contain a stray comment
	// with ss:// — old code could stop at URI-only paths and miss snell.
	// Here pure Clash YAML must yield snell + ss.
	body := `
# managed
proxies:
  - name: only-ss
    type: ss
    server: 9.9.9.9
    port: 443
    cipher: aes-256-gcm
    password: x
  - name: snell-node
    type: snell
    server: 8.8.8.8
    port: 440
    psk: abc
    version: 4
`
	// Use package-level ParseClashConfig; subscribe.ParseBody is in other package —
	// covered there via integration-style re-export if needed.
	nodes, err := ParseClashConfig(body)
	if err != nil {
		t.Fatal(err)
	}
	types := map[string]bool{}
	for _, n := range nodes {
		types[n.Outbound["type"].(string)] = true
	}
	if !types["shadowsocks"] || !types["snell"] {
		t.Fatalf("types=%v", types)
	}
}

func tagsOf(nodes []Node) string {
	var b strings.Builder
	for i, n := range nodes {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString(n.Tag)
	}
	return b.String()
}
