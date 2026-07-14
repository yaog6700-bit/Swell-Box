package config

import (
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/swell-app/swellbox/internal/paths"
)

// Subscription is a saved subscription source.
type Subscription struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type subFile struct {
	Items []Subscription `json:"items"`
}

func subPath() (string, error) {
	dir, err := paths.HomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "subscriptions.json"), nil
}

// LoadSubscriptions returns saved subscription URLs.
func LoadSubscriptions() ([]Subscription, error) {
	p, err := subPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var f subFile
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, err
	}
	return f.Items, nil
}

// SaveSubscriptions writes the full list.
func SaveSubscriptions(items []Subscription) error {
	p, err := subPath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(subFile{Items: items}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o644)
}

// AddSubscription upserts by URL.
func AddSubscription(rawURL string) (Subscription, error) {
	rawURL = strings.TrimSpace(rawURL)
	name := "subscription"
	if u, err := url.Parse(rawURL); err == nil && u.Host != "" {
		name = u.Host
	}
	items, _ := LoadSubscriptions()
	for i, it := range items {
		if it.URL == rawURL {
			items[i].Name = name
			_ = SaveSubscriptions(items)
			return items[i], nil
		}
	}
	s := Subscription{Name: name, URL: rawURL}
	items = append(items, s)
	return s, SaveSubscriptions(items)
}

// RemoveSubscription deletes by URL.
func RemoveSubscription(rawURL string) error {
	items, err := LoadSubscriptions()
	if err != nil {
		return err
	}
	var next []Subscription
	for _, it := range items {
		if it.URL != rawURL {
			next = append(next, it)
		}
	}
	return SaveSubscriptions(next)
}
