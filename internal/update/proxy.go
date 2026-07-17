package update

import (
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Local mixed inbound ports used by seed / common templates.
var localMixedCandidates = []string{
	"http://127.0.0.1:7890",
	"http://127.0.0.1:7080",
}

var (
	proxyMu     sync.Mutex
	proxyCached *url.URL
	proxyChecked time.Time
)

// downloadProxy returns an HTTP proxy for update/API downloads when the local
// sing-box mixed inbound is up. Go does not use Windows "system proxy" registry
// settings — without this, system-proxy mode still dials GitHub directly and
// can hang for a long time, while TUN mode works because it captures all traffic.
func downloadProxy(req *http.Request) (*url.URL, error) {
	if req != nil && req.URL != nil {
		h := strings.ToLower(req.URL.Hostname())
		if h == "127.0.0.1" || h == "localhost" || h == "::1" {
			return nil, nil
		}
	}
	if u := liveLocalMixedProxy(); u != nil {
		return u, nil
	}
	return http.ProxyFromEnvironment(req)
}

func liveLocalMixedProxy() *url.URL {
	proxyMu.Lock()
	defer proxyMu.Unlock()
	if time.Since(proxyChecked) < 2*time.Second {
		return proxyCached
	}
	proxyChecked = time.Now()
	proxyCached = nil
	for _, s := range localMixedCandidates {
		u, err := url.Parse(s)
		if err != nil || u.Host == "" {
			continue
		}
		c, err := net.DialTimeout("tcp", u.Host, 200*time.Millisecond)
		if err != nil {
			continue
		}
		_ = c.Close()
		proxyCached = u
		return u
	}
	return nil
}
