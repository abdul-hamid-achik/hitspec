package clientmgr

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newTestManager(t *testing.T, opts ...Option) *Manager {
	t.Helper()
	dir := t.TempDir()
	base := []Option{
		WithWorkDir(dir),
		WithWatch(false),
		WithHistoryDBPath(filepath.Join(dir, "history.db")),
	}
	m := New(append(base, opts...)...)
	m.Start(context.Background())
	t.Cleanup(func() { _ = m.Close() })
	return m
}

func TestFileCRUDAndWorkspace(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)

	created, err := m.CreateFile(ctx, "api/users.http", "")
	if err != nil {
		t.Fatalf("create file: %v", err)
	}
	if len(created.Requests) != 1 {
		t.Fatalf("created requests = %d, want 1", len(created.Requests))
	}

	raw, err := m.ReadFile(ctx, "api/users.http")
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if !strings.Contains(raw, "GET https://example.com") {
		t.Fatalf("raw content missing default request: %q", raw)
	}

	updated := "### Users\n# @name users\nGET https://example.com/users\n"
	parsed, err := m.SaveFile(ctx, "api/users.http", updated)
	if err != nil {
		t.Fatalf("save file: %v", err)
	}
	if parsed.Requests[0].Name != "users" {
		t.Fatalf("request name = %q, want users", parsed.Requests[0].Name)
	}

	ws, err := m.Workspace(ctx)
	if err != nil {
		t.Fatalf("workspace: %v", err)
	}
	if ws.TotalRequests != 1 || len(ws.Files) != 1 {
		t.Fatalf("workspace = %#v, want one file/request", ws)
	}

	if err := m.DeleteFile(ctx, "api/users.http"); err != nil {
		t.Fatalf("delete file: %v", err)
	}
	if _, err := os.Stat(filepath.Join(m.config.WorkDir, "api/users.http")); !os.IsNotExist(err) {
		t.Fatalf("file still exists or stat failed differently: %v", err)
	}
}

func TestPathTraversalRejected(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)

	if _, err := m.CreateFile(ctx, "../escape.http", "GET https://example.com\n"); err == nil {
		t.Fatal("expected create outside workspace to fail")
	}
	if _, err := m.ReadFile(ctx, "../escape.http"); err == nil {
		t.Fatal("expected read outside workspace to fail")
	}
	if err := m.DeleteFile(ctx, "../escape.http"); err == nil {
		t.Fatal("expected delete outside workspace to fail")
	}
}

func TestReadOnlyBlocksMutations(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t, WithReadOnly(true))

	if _, err := m.CreateFile(ctx, "readonly.http", "GET https://example.com\n"); err == nil {
		t.Fatal("expected create in read-only mode to fail")
	}
	if err := m.SelectEnvironment(ctx, "prod"); err == nil {
		t.Fatal("expected environment mutation in read-only mode to fail")
	}
	if _, err := m.PutConfig(ctx, ConfigDTO{Timeout: 1000}); err == nil {
		t.Fatal("expected config mutation in read-only mode to fail")
	}
}

func TestSubscribeReceivesFileEvents(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m := newTestManager(t)
	ch := m.Subscribe(ctx)

	if _, err := m.CreateFile(ctx, "events.http", "GET https://example.com\n"); err != nil {
		t.Fatalf("create file: %v", err)
	}

	ev := <-ch
	if ev.Type != "file_changed" {
		t.Fatalf("event type = %q, want file_changed", ev.Type)
	}
}
