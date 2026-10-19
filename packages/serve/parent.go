package serve

import (
	"context"
	"os"
	"time"
)

// parentPollInterval is how often the --parent-pid watcher checks its parent.
var parentPollInterval = time.Second

// watchParent calls onGone once, when the process pid has exited, and returns.
// It also returns when ctx ends. alive reports whether a pid is running.
//
// A supervisor that dies without cleanup (SIGKILL, a crash) cannot stop the
// server it started; this lets the server notice and shut itself down. When pid
// is the real parent, a change of os.Getppid (the child was re-parented) counts
// as death too, which is immune to pid reuse.
func watchParent(ctx context.Context, pid int, interval time.Duration, alive func(int) bool, onGone func()) {
	directParent := os.Getppid() == pid
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if !alive(pid) || (directParent && os.Getppid() != pid) {
				onGone()
				return
			}
		}
	}
}
