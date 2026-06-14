package clientmgr

import (
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

const (
	watchDebounce    = 300 * time.Millisecond
	suppressDuration = 2 * time.Second
)

type watchSuppressor struct {
	mu    sync.Mutex
	paths map[string]time.Time
}

func newWatchSuppressor() *watchSuppressor {
	return &watchSuppressor{paths: make(map[string]time.Time)}
}

func (ws *watchSuppressor) suppress(path string) {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	ws.paths[path] = time.Now().Add(suppressDuration)
}

func (ws *watchSuppressor) isSuppressed(path string) bool {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	expiry, ok := ws.paths[path]
	if !ok {
		return false
	}
	if time.Now().After(expiry) {
		delete(ws.paths, path)
		return false
	}
	delete(ws.paths, path)
	return true
}

func (m *Manager) suppressWatch(absPath string) {
	if m.watchSuppress != nil {
		m.watchSuppress.suppress(absPath)
	}
}

func (m *Manager) startWatcher() {
	m.watchSuppress = newWatchSuppressor()
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		m.logger.Error("failed to create file watcher", "error", err)
		return
	}
	_ = filepath.Walk(m.config.WorkDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			_ = watcher.Add(path)
		}
		return nil
	})

	go func() {
		defer watcher.Close()
		timers := make(map[string]*time.Timer)
		for {
			select {
			case <-m.ctx.Done():
				for _, t := range timers {
					t.Stop()
				}
				return
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if !isHitspecFile(event.Name) {
					continue
				}
				if event.Has(fsnotify.Create) {
					if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
						_ = watcher.Add(event.Name)
					}
				}
				if t, exists := timers[event.Name]; exists {
					t.Stop()
				}
				ev := event
				timers[ev.Name] = time.AfterFunc(watchDebounce, func() {
					if m.watchSuppress.isSuppressed(ev.Name) {
						return
					}
					op := "changed"
					if ev.Has(fsnotify.Create) {
						op = "created"
					} else if ev.Has(fsnotify.Remove) || ev.Has(fsnotify.Rename) {
						op = "deleted"
					}
					m.publish("file_changed", FileEvent{
						Path:      m.relPath(ev.Name),
						Operation: op,
						Timestamp: nowISO(),
					})
				})
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				m.logger.Warn("watcher error", "error", err)
			}
		}
	}()
}
