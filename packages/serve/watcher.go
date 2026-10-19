package serve

import (
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/abdul-hamid-achik/hitspec/internal/watchutil"
	"github.com/fsnotify/fsnotify"
)

const watchDebounce = 300 * time.Millisecond
const suppressDuration = 2 * time.Second

// debounceTimer is the pending event for one path. created stays set when a
// Create was seen during the debounce window, so a Create followed by Writes
// (how inotify reports os.WriteFile) is still reported as "created".
type debounceTimer struct {
	timer   *time.Timer
	created bool
}

// suppressedPaths tracks paths that should not trigger watcher broadcasts
// because they were written by the server itself.
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

// suppressWatch marks a file path as server-written so the watcher ignores it.
func (s *Server) suppressWatch(absPath string) {
	if s.watchSuppress != nil {
		s.watchSuppress.suppress(absPath)
	}
}

// startWatcher watches the workDir for .http/.hitspec file changes
// and broadcasts events over the WebSocket hub.
func (s *Server) startWatcher() {
	s.watchSuppress = newWatchSuppressor()

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		s.logger.Error("failed to create file watcher", "error", err)
		return
	}

	if err := watchutil.AddTree(watcher, s.config.WorkDir, nil); err != nil {
		s.logger.Warn("failed to watch some directories", "error", err)
	}

	go func() {
		defer watcher.Close()
		// Per-file timers so concurrent changes to different files
		// don't drop events (previously a single timer was reused).
		timers := make(map[string]*debounceTimer)
		var timersMu sync.Mutex

		// schedule debounces one file event and broadcasts it once the file
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
				// Skip broadcast for server-initiated writes
				if s.watchSuppress.isSuppressed(name) {
					s.logger.Debug("suppressed self-write notification", "path", name)
					return
				}

				operation := watchutil.Operation(op, created)
				relPath, _ := filepath.Rel(s.config.WorkDir, name)
				if relPath == "" {
					relPath = name
				}

				s.hub.Broadcast("file_changed", WSFileEvent{
					Path:      relPath,
					Operation: operation,
					Timestamp: nowISO(),
				})

				s.logger.Info("file changed", "operation", operation, "path", relPath)
			})
			timers[name] = entry
		}

		for {
			select {
			case <-s.ctx.Done():
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
					if info, statErr := os.Stat(event.Name); statErr == nil && info.IsDir() {
						if watchutil.SkipDir(info.Name()) {
							continue
						}
						// Watch the whole new subtree and report files that
						// already exist in it (mkdir -p, cp -R, git checkout).
						if addErr := watchutil.AddTree(watcher, event.Name, func(p string) {
							schedule(p, fsnotify.Create)
						}); addErr != nil {
							s.logger.Warn("failed to watch created directory", "path", event.Name, "error", addErr)
						}
						continue
					}
				}
				if !isHitspecFile(event.Name) {
					continue
				}
				schedule(event.Name, event.Op)
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				s.logger.Warn("watcher error", "error", err)
			}
		}
	}()
}
