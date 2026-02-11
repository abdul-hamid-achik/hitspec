package serve

import (
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

const watchDebounce = 300 * time.Millisecond
const suppressDuration = 2 * time.Second

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

	// Walk and add directories
	_ = filepath.Walk(s.config.WorkDir, func(path string, info os.FileInfo, err error) error {
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
		var timer *time.Timer
		for {
			select {
			case <-s.ctx.Done():
				return
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if !isHitspecFile(event.Name) {
					continue
				}
				if timer != nil {
					timer.Stop()
				}
				ev := event // capture
				timer = time.AfterFunc(watchDebounce, func() {
					// Skip broadcast for server-initiated writes
					if s.watchSuppress.isSuppressed(ev.Name) {
						s.logger.Debug("suppressed self-write notification", "path", ev.Name)
						return
					}

					op := "changed"
					if ev.Has(fsnotify.Create) {
						op = "created"
					} else if ev.Has(fsnotify.Remove) || ev.Has(fsnotify.Rename) {
						op = "deleted"
					}

					relPath, _ := filepath.Rel(s.config.WorkDir, ev.Name)
					if relPath == "" {
						relPath = ev.Name
					}

					s.hub.Broadcast("file_changed", WSFileEvent{
						Path:      relPath,
						Operation: op,
						Timestamp: nowISO(),
					})

					s.logger.Info("file changed", "operation", op, "path", relPath)
				})
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				s.logger.Warn("watcher error", "error", err)
			}
		}
	}()
}
