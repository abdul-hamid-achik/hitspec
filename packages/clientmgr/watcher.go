package clientmgr

import (
	"os"
	"sync"
	"time"

	"github.com/abdul-hamid-achik/hitspec/v2/internal/watchutil"
	"github.com/fsnotify/fsnotify"
)

const (
	watchDebounce    = 300 * time.Millisecond
	suppressDuration = 2 * time.Second
)

// debounceTimer is the pending event for one path. created stays set when a
// Create was seen during the debounce window, so a Create followed by Writes
// (how inotify reports os.WriteFile) is still reported as "created".
type debounceTimer struct {
	timer   *time.Timer
	created bool
}

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
	if err := watchutil.AddTree(watcher, m.config.WorkDir, nil); err != nil {
		m.logger.Warn("failed to watch some directories", "error", err)
	}

	go func() {
		defer watcher.Close()
		timers := make(map[string]*debounceTimer)
		var timersMu sync.Mutex

		// schedule debounces one file event and publishes it once the file
		// has been quiet for watchDebounce.
		schedule := func(name string, op fsnotify.Op) {
			timersMu.Lock()
			defer timersMu.Unlock()
			created := op.Has(fsnotify.Create)
			if prev, exists := timers[name]; exists {
				prev.timer.Stop()
				created = created || prev.created
			}
			entry := &debounceTimer{created: created}
			entry.timer = time.AfterFunc(watchDebounce, func() {
				defer func() {
					timersMu.Lock()
					if timers[name] == entry {
						delete(timers, name)
					}
					timersMu.Unlock()
				}()
				if m.watchSuppress.isSuppressed(name) {
					return
				}
				m.publish("file_changed", FileEvent{
					Path:      m.relPath(name),
					Operation: watchutil.Operation(op, created),
					Timestamp: nowISO(),
				})
			})
			timers[name] = entry
		}

		for {
			select {
			case <-m.ctx.Done():
				timersMu.Lock()
				for _, entry := range timers {
					entry.timer.Stop()
				}
				timers = make(map[string]*debounceTimer)
				timersMu.Unlock()
				return
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Has(fsnotify.Create) {
					if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
						if watchutil.SkipDir(info.Name()) {
							continue
						}
						// Watch the whole new subtree and report files that
						// already exist in it (mkdir -p, cp -R, git checkout).
						if addErr := watchutil.AddTree(watcher, event.Name, func(p string) {
							schedule(p, fsnotify.Create)
						}); addErr != nil {
							m.logger.Warn("failed to watch created directory", "path", event.Name, "error", addErr)
						}
						continue
					}
				}
				if !isHitspecFile(event.Name) {
					continue
				}
				// Drop the cached parse now: the debounced event (and Manager's own
				// writes, which the watcher suppresses) may come later or never.
				m.search.invalidate(event.Name)
				schedule(event.Name, event.Op)
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				m.logger.Warn("watcher error", "error", err)
			}
		}
	}()
}
