package clashapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// DefaultAddr is injected into runtime config.
const DefaultAddr = "127.0.0.1:9090"

// Client talks to sing-box Clash API.
type Client struct {
	Base   string
	Secret string
	HTTP   *http.Client
}

func New(addr string) *Client {
	if addr == "" {
		addr = DefaultAddr
	}
	return &Client{
		Base: "http://" + addr,
		HTTP: &http.Client{Timeout: 5 * time.Second},
	}
}

func (c *Client) do(method, path string, body any) ([]byte, error) {
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, c.Base+path, rdr)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.Secret != "" {
		req.Header.Set("Authorization", "Bearer "+c.Secret)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return data, fmt.Errorf("clash api %s %s: %s", method, path, resp.Status)
	}
	return data, nil
}

// Proxies returns proxy group map from Clash API.
func (c *Client) Proxies() (map[string]any, error) {
	data, err := c.do(http.MethodGet, "/proxies", nil)
	if err != nil {
		return nil, err
	}
	var root struct {
		Proxies map[string]any `json:"proxies"`
	}
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, err
	}
	return root.Proxies, nil
}

// Select sets the current node for a selector group.
func (c *Client) Select(group, name string) error {
	path := "/proxies/" + url.PathEscape(group)
	_, err := c.do(http.MethodPut, path, map[string]string{"name": name})
	return err
}

// GroupNow returns selected now and all for a Selector group.
func (c *Client) GroupNow(group string) (now string, all []string, err error) {
	proxies, err := c.Proxies()
	if err != nil {
		return "", nil, err
	}
	raw, ok := proxies[group]
	if !ok {
		return "", nil, fmt.Errorf("group %q not found", group)
	}
	m, ok := raw.(map[string]any)
	if !ok {
		return "", nil, fmt.Errorf("bad group payload")
	}
	now, _ = m["now"].(string)
	if arr, ok := m["all"].([]any); ok {
		for _, v := range arr {
			if s, ok := v.(string); ok {
				all = append(all, s)
			}
		}
	}
	return now, all, nil
}
