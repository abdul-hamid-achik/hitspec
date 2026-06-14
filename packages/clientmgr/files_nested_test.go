package clientmgr

import (
	"context"
	"testing"
)

func TestRenameFileCreatesNestedDir(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	if _, err := m.CreateFile(ctx, "old.http", "### Ping\nGET https://example.com\n"); err != nil {
		t.Fatalf("create: %v", err)
	}
	parsed, err := m.RenameFile(ctx, "old.http", "api/v1/renamed.http")
	if err != nil {
		t.Fatalf("rename into nested dir: %v", err)
	}
	if len(parsed.Requests) != 1 {
		t.Fatalf("want 1 request, got %d", len(parsed.Requests))
	}
	if _, err := m.ReadFile(ctx, "api/v1/renamed.http"); err != nil {
		t.Fatalf("nested file should exist: %v", err)
	}
}

func TestCopyFileCreatesNestedDir(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	if _, err := m.CreateFile(ctx, "src.http", "### Ping\nGET https://example.com\n"); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := m.CopyFile(ctx, "src.http", "sub/dir/copy.http"); err != nil {
		t.Fatalf("copy into nested dir: %v", err)
	}
	if _, err := m.ReadFile(ctx, "sub/dir/copy.http"); err != nil {
		t.Fatalf("nested copy should exist: %v", err)
	}
}
