package sharelink

import (
	"net/url"
	"strconv"
	"strings"
)

// buildTLS builds a sing-box outbound TLS object from common share-link query keys.
// security: "", "none", "tls", "reality", "xtls" (treated as tls).
func buildTLS(security, sni, fp, alpn string, insecure bool, pbk, sid, spx string) map[string]any {
	sec := strings.ToLower(strings.TrimSpace(security))
	switch sec {
	case "", "none", "0", "false":
		// still allow explicit insecure/sni-only if caller forces enabled elsewhere
		if !insecure && sni == "" && pbk == "" {
			return nil
		}
		if sec == "none" || sec == "0" || sec == "false" {
			if pbk == "" {
				return nil
			}
		}
	}

	// Reality implies TLS
	if pbk != "" || sec == "reality" {
		sec = "reality"
	} else if sec == "xtls" || sec == "tls" || sec == "1" || sec == "true" || sni != "" || insecure || alpn != "" {
		sec = "tls"
	} else {
		return nil
	}

	tls := map[string]any{
		"enabled": true,
	}
	if sni != "" {
		tls["server_name"] = sni
	}
	if insecure {
		tls["insecure"] = true
	}
	if alpnList := parseALPN(alpn); len(alpnList) > 0 {
		tls["alpn"] = alpnList
	}
	if fp == "" {
		fp = "chrome"
	}
	tls["utls"] = map[string]any{
		"enabled":     true,
		"fingerprint": fp,
	}
	if sec == "reality" {
		reality := map[string]any{
			"enabled":    true,
			"public_key": pbk,
		}
		if sid != "" {
			reality["short_id"] = sid
		}
		if spx != "" {
			// sing-box uses short_id; spider_x is xray-specific — ignore if unknown
			_ = spx
		}
		tls["reality"] = reality
	}
	return tls
}

func buildTLSFromQuery(q url.Values, defaultSecurity string) map[string]any {
	security := queryFirst(q, "security", "tls")
	if security == "" {
		security = defaultSecurity
	}
	// hysteria-style peer / sni
	sni := queryFirst(q, "sni", "peer", "servername", "host")
	// for some links host is SNI only when security set; keep as-is
	if security == "" && (q.Get("sni") != "" || q.Get("peer") != "") {
		security = "tls"
	}
	fp := queryFirst(q, "fp", "fingerprint")
	alpn := queryFirst(q, "alpn")
	insecure := queryBool(q, "insecure", "allowInsecure", "allow_insecure", "skip-cert-verify")
	pbk := queryFirst(q, "pbk", "publicKey", "public-key", "public_key")
	sid := queryFirst(q, "sid", "shortId", "short_id", "short-id")
	spx := queryFirst(q, "spx", "spiderX")
	// Reality without security=reality
	if pbk != "" && security == "" {
		security = "reality"
	}
	// tls=1 / tls=tls style (vmess json uses separate field; query may use tls=)
	if tlsFlag := strings.ToLower(q.Get("tls")); tlsFlag == "1" || tlsFlag == "true" || tlsFlag == "tls" {
		if security == "" {
			security = "tls"
		}
	}
	return buildTLS(security, sni, fp, alpn, insecure, pbk, sid, spx)
}

// buildV2RayTransport maps share-link type/network params to sing-box transport.
// Returns nil for plain TCP without headers.
func buildV2RayTransport(network string, q url.Values) map[string]any {
	netw := strings.ToLower(strings.TrimSpace(network))
	if netw == "" {
		netw = "tcp"
	}
	// normalize aliases
	switch netw {
	case "websocket":
		netw = "ws"
	case "http2", "h2":
		netw = "http"
	case "gun":
		netw = "grpc"
	case "tcphttp", "tcp-http":
		netw = "tcp"
	case "xhttp", "splithttp":
		// not a first-class sing-box transport in all versions; best-effort as httpupgrade
		netw = "httpupgrade"
	}

	headerType := strings.ToLower(queryFirst(q, "headerType", "header_type"))
	path := queryFirst(q, "path", "spx")
	host := queryFirst(q, "host", "authority")
	serviceName := queryFirst(q, "serviceName", "service_name", "servicename")
	mode := queryFirst(q, "mode")

	switch netw {
	case "tcp", "raw", "none":
		if headerType == "http" {
			t := map[string]any{"type": "http"}
			if path != "" {
				t["path"] = path
			}
			if host != "" {
				t["host"] = []any{host}
			}
			return t
		}
		return nil

	case "ws":
		t := map[string]any{"type": "ws"}
		// path may be "/path?ed=2048"
		maxEarly := 0
		earlyHeader := ""
		if path != "" {
			if i := strings.Index(path, "?"); i >= 0 {
				rawQ := path[i+1:]
				path = path[:i]
				if v, err := url.ParseQuery(rawQ); err == nil {
					if ed := v.Get("ed"); ed != "" {
						if n, err := strconv.Atoi(ed); err == nil {
							maxEarly = n
							earlyHeader = "Sec-WebSocket-Protocol"
						}
					}
				}
			}
			if path != "" {
				t["path"] = path
			}
		}
		if host != "" {
			t["headers"] = map[string]any{"Host": host}
		}
		if maxEarly > 0 {
			t["max_early_data"] = maxEarly
			t["early_data_header_name"] = earlyHeader
		}
		return t

	case "http":
		t := map[string]any{"type": "http"}
		if path != "" {
			t["path"] = path
		}
		if host != "" {
			// may be comma-separated
			parts := strings.Split(host, ",")
			var hosts []any
			for _, p := range parts {
				p = strings.TrimSpace(p)
				if p != "" {
					hosts = append(hosts, p)
				}
			}
			if len(hosts) > 0 {
				t["host"] = hosts
			}
		}
		if method := queryFirst(q, "method"); method != "" {
			t["method"] = method
		}
		return t

	case "httpupgrade":
		t := map[string]any{"type": "httpupgrade"}
		if path != "" {
			t["path"] = path
		}
		if host != "" {
			t["host"] = host
		}
		return t

	case "grpc":
		t := map[string]any{"type": "grpc"}
		if serviceName != "" {
			t["service_name"] = serviceName
		} else if path != "" {
			// some clients put service name in path
			t["service_name"] = strings.TrimPrefix(path, "/")
		}
		if mode != "" {
			// multi mode is xray; sing-box uses different defaults — ignore safely
			_ = mode
		}
		return t

	case "quic":
		return map[string]any{"type": "quic"}

	default:
		return nil
	}
}

func applyTransport(ob map[string]any, network string, q url.Values) {
	if t := buildV2RayTransport(network, q); t != nil {
		ob["transport"] = t
	}
}

func applyTLS(ob map[string]any, q url.Values, defaultSecurity string) {
	if tls := buildTLSFromQuery(q, defaultSecurity); tls != nil {
		ob["tls"] = tls
	}
}

func applyPacketEncoding(ob map[string]any, q url.Values) {
	pe := strings.ToLower(queryFirst(q, "packetEncoding", "packet_encoding", "packetencoding"))
	switch pe {
	case "xudp", "packetaddr", "none":
		if pe == "none" {
			return
		}
		ob["packet_encoding"] = pe
	case "":
		// leave default for protocol
	default:
		ob["packet_encoding"] = pe
	}
}

func applyFlow(ob map[string]any, q url.Values) {
	if flow := queryFirst(q, "flow"); flow != "" && !strings.EqualFold(flow, "none") {
		ob["flow"] = flow
	}
}

func applyMultiplex(ob map[string]any, q url.Values) {
	// mux=1 style (common in some clients)
	if queryBool(q, "mux", "multiplex") {
		ob["multiplex"] = map[string]any{"enabled": true}
	}
}
