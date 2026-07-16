package config

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/swell-app/swellbox/internal/paths"
)

// Subscription is a saved subscription source.
// NodeTags records outbound tags last imported from this URL, so deleting
// the subscription can also remove those nodes from the active config.
type Subscription struct {
	Name     string   `json:"name"`
	URL      string   `json:"url"`
	NodeTags []string `json:"node_tags,omitempty"`
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

// AddSubscription upserts by URL (keeps existing NodeTags if any).
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

// SetSubscriptionNodeTags records which config node tags came from this URL.
func SetSubscriptionNodeTags(rawURL string, tags []string) error {
	rawURL = strings.TrimSpace(rawURL)
	items, err := LoadSubscriptions()
	if err != nil {
		return err
	}
	for i, it := range items {
		if it.URL != rawURL {
			continue
		}
		// de-dup empty
		var clean []string
		seen := map[string]bool{}
		for _, t := range tags {
			t = strings.TrimSpace(t)
			if t == "" || seen[t] {
				continue
			}
			seen[t] = true
			clean = append(clean, t)
		}
		items[i].NodeTags = clean
		return SaveSubscriptions(items)
	}
	return fmt.Errorf("subscription not found")
}

// RemoveSubscription deletes by URL and returns the removed entry (for node cleanup).
func RemoveSubscription(rawURL string) (*Subscription, error) {
	rawURL = strings.TrimSpace(rawURL)
	items, err := LoadSubscriptions()
	if err != nil {
		return nil, err
	}
	var (
		next    []Subscription
		removed *Subscription
	)
	for _, it := range items {
		if it.URL == rawURL {
			cp := it
			removed = &cp
			continue
		}
		next = append(next, it)
	}
	if removed == nil {
		return nil, fmt.Errorf("subscription not found")
	}
	if err := SaveSubscriptions(next); err != nil {
		return nil, err
	}
	return removed, nil
}

// GetSubscription returns the saved entry for rawURL, or nil.
func GetSubscription(rawURL string) *Subscription {
	items, err := LoadSubscriptions()
	if err != nil {
		return nil
	}
	rawURL = strings.TrimSpace(rawURL)
	for i := range items {
		if items[i].URL == rawURL {
			cp := items[i]
			return &cp
		}
	}
	return nil
}
