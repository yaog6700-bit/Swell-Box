package watch

import (
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// ConfigWatcher debounces writes to the active config file.
type ConfigWatcher struct {
	mu       sync.Mutex
	w        *fsnotify.Watcher
	path     string
	callback func()
	stopCh   chan struct{}
	timer    *time.Timer
}

// New creates a watcher; call Watch(path) then Start.
func New(onChange func()) (*ConfigWatcher, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	cw := &ConfigWatcher{
		w:        w,
		callback: onChange,
		stopCh:   make(chan struct{}),
	}
	return cw, nil
}

// SetPath watches a single config file (replaces previous).
func (c *ConfigWatcher) SetPath(path string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.path != "" {
		_ = c.w.Remove(filepath.Dir(c.path))
	}
	c.path = path
	// Watch directory so renames/atomic saves are seen.
	dir := filepath.Dir(path)
	return c.w.Add(dir)
}

// Start begins the event loop.
func (c *ConfigWatcher) Start() {
	go func() {
		for {
			select {
			case <-c.stopCh:
				return
			case ev, ok := <-c.w.Events:
				if !ok {
					return
				}
				c.mu.Lock()
				target := c.path
				c.mu.Unlock()
				if target == "" {
					continue
				}
				if filepath.Clean(ev.Name) != filepath.Clean(target) {
					continue
				}
				if ev.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename) == 0 {
					continue
				}
				c.debounce()
			case _, ok := <-c.w.Errors:
				if !ok {
					return
				}
			}
		}
	}()
}

func (c *ConfigWatcher) debounce() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.timer != nil {
		c.timer.Stop()
	}
	c.timer = time.AfterFunc(800*time.Millisecond, func() {
		if c.callback != nil {
			c.callback()
		}
	})
}

// Close stops the watcher.
func (c *ConfigWatcher) Close() error {
	close(c.stopCh)
	c.mu.Lock()
	if c.timer != nil {
		c.timer.Stop()
	}
	c.mu.Unlock()
	return c.w.Close()
}
