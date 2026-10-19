package serve

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// startWatchedServer runs only the file watcher of a Server and returns a
// channel that receives every hub broadcast.
func startWatchedServer(t *testing.T) (string, chan []byte) {
	t.Helper()
	dir := t.TempDir()
	s := newTestServer(t, WithWorkDir(dir))
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	s.ctx = ctx
	client := &Client{send: make(chan []byte, 64)}
	s.hub.register(client)
	s.startWatcher()
	return dir, client.send
}

func waitForBroadcast(t *testing.T, msgs <-chan []byte, wantPath, wantOp string) {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		select {
		case raw := <-msgs:
			var msg struct {
				Type    string      `json:"type"`
				Payload WSFileEvent `json:"payload"`
			}
			if err := json.Unmarshal(raw, &msg); err != nil || msg.Type != "file_changed" {
				continue
			}
			if filepath.ToSlash(msg.Payload.Path) == wantPath && msg.Payload.Operation == wantOp {
				return
			}
		case <-deadline:
			t.Fatalf("timed out waiting for %s broadcast on %s", wantOp, wantPath)
		}
	}
}

func TestWatcherBroadcastsFilesInNewNestedDirs(t *testing.T) {
	dir, msgs := startWatchedServer(t)

	nested := filepath.Join(dir, "x", "y")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	time.Sleep(200 * time.Millisecond)
	if err := os.WriteFile(filepath.Join(nested, "n.http"), []byte("GET http://x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	waitForBroadcast(t, msgs, "x/y/n.http", "created")
}

func TestWatcherBroadcastsFilesInDirMovedIntoWorkspace(t *testing.T) {
	dir, msgs := startWatchedServer(t)

	staging := filepath.Join(t.TempDir(), "pkg")
	if err := os.MkdirAll(staging, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(staging, "m.http"), []byte("GET http://x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(staging, filepath.Join(dir, "pkg")); err != nil {
		t.Fatal(err)
	}
	waitForBroadcast(t, msgs, "pkg/m.http", "created")
}
