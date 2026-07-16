package sharelink

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestParseSS(t *testing.T) {
	// method:password@host:port fully base64
	raw := base64.StdEncoding.EncodeToString([]byte("aes-256-gcm:secret@1.2.3.4:8388"))
	nodes, err := Parse("ss://" + raw + "#MySS")
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 1 || nodes[0].Outbound["type"] != "shadowsocks" {
		t.Fatalf("got %+v", nodes)
	}
	if nodes[0].Outbound["server"] != "1.2.3.4" {
		t.Fatalf("server=%v", nodes[0].Outbound["server"])
	}
	if nodes[0].Tag != "MySS" {
		t.Fatalf("tag=%s", nodes[0].Tag)
	}
}

func TestParseSSUserInfo(t *testing.T) {
	user := base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString([]byte("chacha20-ietf-poly1305:pass"))
	// url-safe base64 without padding
	link := "ss://" + user + "@example.com:443#tag"
	nodes, err := Parse(link)
	if err != nil {
		t.Fatal(err)
	}
	ob := nodes[0].Outbound
	if ob["method"] != "chacha20-ietf-poly1305" || ob["password"] != "pass" {
		t.Fatalf("method/pass=%v/%v", ob["method"], ob["password"])
	}
}

func TestParseVMessJSON(t *testing.T) {
	js := `{"v":"2","ps":"HK-01","add":"a.example.com","port":"443","id":"11111111-1111-1111-1111-111111111111","aid":"0","scy":"auto","net":"ws","type":"none","host":"a.example.com","path":"/ws","tls":"tls","sni":"a.example.com","fp":"chrome"}`
	link := "vmess://" + base64.StdEncoding.EncodeToString([]byte(js))
	nodes, err := Parse(link)
	if err != nil {
		t.Fatal(err)
	}
	ob := nodes[0].Outbound
	if ob["type"] != "vmess" {
		t.Fatalf("type=%v", ob["type"])
	}
	if ob["server"] != "a.example.com" {
		t.Fatalf("server=%v", ob["server"])
	}
	tr, _ := ob["transport"].(map[string]any)
	if tr == nil || tr["type"] != "ws" {
		t.Fatalf("transport=%v", ob["transport"])
	}
	tls, _ := ob["tls"].(map[string]any)
	if tls == nil || tls["enabled"] != true {
		t.Fatalf("tls=%v", ob["tls"])
	}
}

func TestParseTUICGluedUUIDPassword(t *testing.T) {
	// Username incorrectly contains "uuid:password" as one field (URL-encoded colon).
	link := "tuic://33333333-3333-4333-8333-333333333333%3Abluesky333@i.coffee.bbroot.com:55543?congestion_control=bbr#TUIC-BBroot"
	nodes, err := Parse(link)
	if err != nil {
		t.Fatal(err)
	}
	ob := nodes[0].Outbound
	if ob["uuid"] != "33333333-3333-4333-8333-333333333333" {
		t.Fatalf("uuid=%v", ob["uuid"])
	}
	if ob["password"] != "bluesky333" {
		t.Fatalf("password=%v", ob["password"])
	}
}

func TestNormalizeUUID(t *testing.T) {
	ok, err := normalizeUUID("11111111-1111-1111-1111-111111111111")
	if err != nil || ok != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("dashed: %v %v", ok, err)
	}
	ok, err = normalizeUUID("11111111111111111111111111111111")
	if err != nil || ok != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("hex32: %v %v", ok, err)
	}
	if _, err := normalizeUUID("not-a-uuid"); err == nil {
		t.Fatal("expected invalid")
	}
	if _, err := normalizeUUID(""); err == nil {
		t.Fatal("expected empty invalid")
	}
}

func TestParseVLESSBadUUID(t *testing.T) {
	_, err := Parse("vless://bad-id@1.2.3.4:443?encryption=none#x")
	if err == nil {
		t.Fatal("expected invalid uuid error")
	}
	if !strings.Contains(err.Error(), "uuid") {
		t.Fatalf("err=%v", err)
	}
}

func TestParseVLESSReality(t *testing.T) {
	link := "vless://11111111-1111-1111-1111-111111111111@1.2.3.4:443?encryption=none&flow=xtls-rprx-vision&security=reality&sni=www.example.com&fp=chrome&pbk=PUBLICKEY&sid=abcd&type=tcp#RL"
	nodes, err := Parse(link)
	if err != nil {
		t.Fatal(err)
	}
	ob := nodes[0].Outbound
	if ob["type"] != "vless" || ob["flow"] != "xtls-rprx-vision" {
		t.Fatalf("%+v", ob)
	}
	tls, _ := ob["tls"].(map[string]any)
	if tls == nil {
		t.Fatal("missing tls")
	}
	reality, _ := tls["reality"].(map[string]any)
	if reality == nil || reality["public_key"] != "PUBLICKEY" {
		t.Fatalf("reality=%v", reality)
	}
}

func TestParseVLESSWS(t *testing.T) {
	link := "vless://11111111-1111-1111-1111-111111111111@1.2.3.4:443?type=ws&security=tls&sni=cdn.example.com&path=%2Fws&host=cdn.example.com#ws1"
	nodes, err := Parse(link)
	if err != nil {
		t.Fatal(err)
	}
	tr, _ := nodes[0].Outbound["transport"].(map[string]any)
	if tr["type"] != "ws" || tr["path"] != "/ws" {
		t.Fatalf("transport=%v", tr)
	}
}

func TestParseTrojan(t *testing.T) {
	link := "trojan://p%40ss@host.example:443?security=tls&sni=host.example&type=ws&path=%2Ft#TJ"
	nodes, err := Parse(link)
	if err != nil {
		t.Fatal(err)
	}
	ob := nodes[0].Outbound
	if ob["type"] != "trojan" || ob["password"] != "p@ss" {
		t.Fatalf("%+v", ob)
	}
	if _, ok := ob["tls"]; !ok {
		t.Fatal("expected tls")
	}
}

func TestParseHysteria2(t *testing.T) {
	link := "hysteria2://secret@1.2.3.4:8443?sni=www.bing.com&obfs=salamander&obfs-password=obfspass&insecure=1#HY2"
	nodes, err := Parse(link)
	if err != nil {
		t.Fatal(err)
	}
	ob := nodes[0].Outbound
	if ob["type"] != "hysteria2" || ob["password"] != "secret" {
		t.Fatalf("%+v", ob)
	}
	obfs, _ := ob["obfs"].(map[string]any)
	if obfs["type"] != "salamander" || obfs["password"] != "obfspass" {
		t.Fatalf("obfs=%v", obfs)
	}
}

func TestParseHy2Alias(t *testing.T) {
	link := "hy2://pwd@example.com:443?sni=example.com#x"
	nodes, err := Parse(link)
	if err != nil {
		t.Fatal(err)
	}
	if nodes[0].Outbound["type"] != "hysteria2" {
		t.Fatal(nodes[0].Outbound["type"])
	}
}

func TestParseHysteria(t *testing.T) {
	link := "hysteria://1.2.3.4:36712?auth=hello&peer=sni.example&upmbps=50&downmbps=100&insecure=1#H1"
	nodes, err := Parse(link)
	if err != nil {
		t.Fatal(err)
	}
	ob := nodes[0].Outbound
	if ob["type"] != "hysteria" || ob["auth_str"] != "hello" {
		t.Fatalf("%+v", ob)
	}
	if ob["up_mbps"] != 50 || ob["down_mbps"] != 100 {
		t.Fatalf("bw up=%v down=%v", ob["up_mbps"], ob["down_mbps"])
	}
}

func TestParseTUIC(t *testing.T) {
	link := "tuic://11111111-1111-1111-1111-111111111111:pass@1.2.3.4:443?congestion_control=bbr&udp_relay_mode=native&sni=example.com&alpn=h3#T"
	nodes, err := Parse(link)
	if err != nil {
		t.Fatal(err)
	}
	ob := nodes[0].Outbound
	if ob["type"] != "tuic" || ob["password"] != "pass" {
		t.Fatalf("%+v", ob)
	}
	if ob["congestion_control"] != "bbr" {
		t.Fatalf("cc=%v", ob["congestion_control"])
	}
}

func TestParseAnyTLS(t *testing.T) {
	link := "anytls://mypass@1.2.3.4:443?sni=example.com#A"
	nodes, err := Parse(link)
	if err != nil {
		t.Fatal(err)
	}
	if nodes[0].Outbound["type"] != "anytls" {
		t.Fatal(nodes[0].Outbound)
	}
}

func TestParseSOCKSAndHTTP(t *testing.T) {
	nodes, err := Parse("socks5://user:pass@1.2.3.4:1080#S\nhttp://user:pass@1.2.3.4:8080#H")
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 2 {
		t.Fatalf("len=%d", len(nodes))
	}
	if nodes[0].Outbound["type"] != "socks" || nodes[1].Outbound["type"] != "http" {
		t.Fatalf("%v %v", nodes[0].Outbound["type"], nodes[1].Outbound["type"])
	}
}

func TestParseHTTPRejectsSubscriptionURL(t *testing.T) {
	_, err := Parse("https://example.com/api/v1/sub?token=abcdef")
	if err == nil {
		t.Fatal("expected error for subscription-like URL")
	}
}

func TestParseWireGuard(t *testing.T) {
	// dummy keys (format only)
	priv := base64.StdEncoding.EncodeToString(make([]byte, 32))
	pub := base64.StdEncoding.EncodeToString(make([]byte, 32))
	link := "wireguard://" + urlQueryEscape(priv) + "@1.2.3.4:51820?publickey=" + urlQueryEscape(pub) + "&address=10.0.0.2/32&mtu=1420#WG"
	nodes, err := Parse(link)
	if err != nil {
		t.Fatal(err)
	}
	if nodes[0].Outbound["type"] != "wireguard" {
		t.Fatal(nodes[0].Outbound)
	}
}

func TestParseSnellNaiveSSH(t *testing.T) {
	cases := []struct {
		link string
		typ  string
	}{
		{"snell://psk123@1.2.3.4:440#S", "snell"},
		{"naive+https://u:p@1.2.3.4:443#N", "naive"},
		{"ssh://root:pwd@1.2.3.4:22#SSH", "ssh"},
	}
	for _, c := range cases {
		nodes, err := Parse(c.link)
		if err != nil {
			t.Fatalf("%s: %v", c.link, err)
		}
		if nodes[0].Outbound["type"] != c.typ {
			t.Fatalf("%s type=%v", c.link, nodes[0].Outbound["type"])
		}
	}
}

func TestParseSSRRejected(t *testing.T) {
	_, err := Parse("ssr://dGVzdA==")
	if err == nil || !strings.Contains(err.Error(), "not supported") {
		t.Fatalf("err=%v", err)
	}
}

func TestParseNaiveHTTPSShare(t *testing.T) {
	// Common real-world form (not naive+https://)
	link := "https://user123:pass123@np.128128.best:443#naive"
	nodes, err := Parse(link)
	if err != nil {
		t.Fatal(err)
	}
	ob := nodes[0].Outbound
	if ob["type"] != "naive" {
		t.Fatalf("type=%v want naive", ob["type"])
	}
	if nodes[0].Tag != "naive" {
		t.Fatalf("tag=%s", nodes[0].Tag)
	}
	if ob["server"] != "np.128128.best" || ob["server_port"] != 443 {
		t.Fatalf("server=%v:%v", ob["server"], ob["server_port"])
	}
	if ob["username"] != "user123" || ob["password"] != "pass123" {
		t.Fatalf("auth=%v/%v", ob["username"], ob["password"])
	}
	tls, _ := ob["tls"].(map[string]any)
	if tls == nil || tls["enabled"] != true || tls["server_name"] != "np.128128.best" {
		t.Fatalf("tls=%v", ob["tls"])
	}
}

func TestParseClashSnell6(t *testing.T) {
	// Real-world Clash classical line (Snell v6)
	line := "Snell6 = snell, 216.236.7.38, 26005, psk = tRA3iO4bPnHz6rPhojK1r, version = 6, reuse = true, tfo = true"
	nodes, err := Parse(line)
	if err != nil {
		t.Fatal(err)
	}
	ob := nodes[0].Outbound
	if ob["type"] != "snell" {
		t.Fatalf("type=%v", ob["type"])
	}
	if nodes[0].Tag != "Snell6" {
		t.Fatalf("tag=%s", nodes[0].Tag)
	}
	if ob["server"] != "216.236.7.38" || ob["server_port"] != 26005 {
		t.Fatalf("server=%v:%v", ob["server"], ob["server_port"])
	}
	if ob["psk"] != "tRA3iO4bPnHz6rPhojK1r" {
		t.Fatalf("psk=%v", ob["psk"])
	}
	if ob["version"] != 6 {
		t.Fatalf("version=%v", ob["version"])
	}
	if ob["reuse"] != true {
		t.Fatalf("reuse=%v", ob["reuse"])
	}
	if ob["tcp_fast_open"] != true {
		t.Fatalf("tfo=%v", ob["tcp_fast_open"])
	}
}

func TestParseMultiLinePartial(t *testing.T) {
	raw := "not-a-link\nss://YWVzLTI1Ni1nY206c2VjcmV0QDEuMi4zLjQ6ODM4OA==#ok\nbogus://"
	nodes, err := Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 1 {
		t.Fatalf("len=%d", len(nodes))
	}
}

func urlQueryEscape(s string) string {
	// minimal escape for test keys
	return strings.ReplaceAll(s, "+", "%2B")
}
