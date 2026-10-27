package dashboard

import (
	_ "embed"
	"fmt"
	"net"
	"net/http"
	"sync"
)

//go:embed index.html
var indexHTML []byte

var (
	once sync.Once
	port = 9092
)

// EnsureServer starts the local offline dashboard server on 127.0.0.1:9092.
// Returns the actual listening port.
func EnsureServer() int {
	once.Do(func() {
		l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err != nil {
			l, err = net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				return
			}
			port = l.Addr().(*net.TCPAddr).Port
		}
		mux := http.NewServeMux()
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Access-Control-Allow-Origin", "*")
			_, _ = w.Write(indexHTML)
		})
		server := &http.Server{Handler: mux}
		go func() {
			_ = server.Serve(l)
		}()
	})
	return port
}

// LocalDashboardURL returns the URL to access the built-in local dashboard.
func LocalDashboardURL(apiPort int, secret string) string {
	p := EnsureServer()
	return fmt.Sprintf("http://127.0.0.1:%d/?port=%d&secret=%s", p, apiPort, secret)
}
