package subscribe

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestParseBodyClashYAMLWithSnell(t *testing.T) {
	body := `
proxies:
  - name: SS1
    type: ss
    server: 1.2.3.4
    port: 8388
    cipher: aes-256-gcm
    password: secret
  - name: Snell1
    type: snell
    server: 5.6.7.8
    port: 440
    psk: mypsk
    version: 6
    reuse: true
`
	nodes, err := ParseBody(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 2 {
		t.Fatalf("want 2 got %d", len(nodes))
	}
	var hasSS, hasSnell bool
	for _, n := range nodes {
		switch n.Outbound["type"] {
		case "shadowsocks":
			hasSS = true
		case "snell":
			hasSnell = true
		}
	}
	if !hasSS || !hasSnell {
		t.Fatalf("ss=%v snell=%v nodes=%+v", hasSS, hasSnell, nodes)
	}
}

func TestParseBodyBase64URIList(t *testing.T) {
	raw := "ss://YWVzLTI1Ni1nY206c2VjcmV0QDEuMi4zLjQ6ODM4OA==#SS\n" +
		"snell://psk123@9.9.9.9:440?version=4#SN"
	enc := base64.StdEncoding.EncodeToString([]byte(raw))
	nodes, err := ParseBody(enc)
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) < 2 {
		t.Fatalf("want >=2 got %d %+v", len(nodes), nodes)
	}
	var hasSS, hasSnell bool
	for _, n := range nodes {
		switch n.Outbound["type"] {
		case "shadowsocks":
			hasSS = true
		case "snell":
			hasSnell = true
		}
	}
	if !hasSS || !hasSnell {
		t.Fatalf("ss=%v snell=%v", hasSS, hasSnell)
	}
}

func TestParseBodyPlainMixedLines(t *testing.T) {
	body := strings.Join([]string{
		"ss://YWVzLTI1Ni1nY206c2VjcmV0QDEuMi4zLjQ6ODM4OA==#SS",
		"Snell6 = snell, 216.236.7.38, 26005, psk = abcdef, version = 6, reuse = true",
	}, "\n")
	nodes, err := ParseBody(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 2 {
		t.Fatalf("got %d", len(nodes))
	}
}
