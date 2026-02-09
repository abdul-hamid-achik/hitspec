package serve

import (
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
)

const watchDebounce = 300 * time.Millisecond

// startWatcher watches the workDir for .http/.hitspec file changes
// and broadcasts events over the WebSocket hub.
func (s *Server) startWatcher() {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Printf("failed to create file watcher: %v", err)
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

					s.hub.Broadcast("file:"+op, WSFileEvent{
						Path:      relPath,
						Operation: op,
						Timestamp: nowISO(),
					})

					if s.config.Verbose {
						log.Printf("file %s: %s", op, relPath)
					}
				})
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Printf("watcher error: %v", err)
			}
		}
	}()
}
