package clientmgr

import (
	"context"
	"testing"
)

func TestRenameFile(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)

	if _, err := m.CreateFile(ctx, "old.http", "### Ping\nGET https://example.com\n"); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := m.RenameFile(ctx, "old.http", "renamed.http"); err != nil {
		t.Fatalf("rename: %v", err)
	}
	if _, err := m.ReadFile(ctx, "renamed.http"); err != nil {
		t.Fatalf("renamed file should exist: %v", err)
	}
	if _, err := m.ReadFile(ctx, "old.http"); err == nil {
		t.Fatal("old file should no longer exist")
	}
}

func TestRenameFileRejectsExistingDestination(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	_, _ = m.CreateFile(ctx, "a.http", "GET https://a\n")
	_, _ = m.CreateFile(ctx, "b.http", "GET https://b\n")
	if _, err := m.RenameFile(ctx, "a.http", "b.http"); err == nil {
		t.Fatal("rename onto an existing file should fail")
	}
}

func TestRenameFileRejectsBadExtension(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	_, _ = m.CreateFile(ctx, "a.http", "GET https://a\n")
	if _, err := m.RenameFile(ctx, "a.http", "a.txt"); err == nil {
		t.Fatal("rename to a non-hitspec extension should fail")
	}
}

func TestCopyFile(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	content := "### Ping\nGET https://example.com\n"
	if _, err := m.CreateFile(ctx, "src.http", content); err != nil {
		t.Fatalf("create: %v", err)
	}
	parsed, err := m.CopyFile(ctx, "src.http", "copy.http")
	if err != nil {
		t.Fatalf("copy: %v", err)
	}
	if len(parsed.Requests) != 1 {
		t.Fatalf("copy parsed %d requests, want 1", len(parsed.Requests))
	}
	// Both files exist with identical content.
	src, _ := m.ReadFile(ctx, "src.http")
	dst, _ := m.ReadFile(ctx, "copy.http")
	if src != dst {
		t.Fatalf("copy content mismatch:\nsrc=%q\ndst=%q", src, dst)
	}
}

func TestCopyFileRejectsExistingDestination(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	_, _ = m.CreateFile(ctx, "a.http", "GET https://a\n")
	_, _ = m.CreateFile(ctx, "b.http", "GET https://b\n")
	if _, err := m.CopyFile(ctx, "a.http", "b.http"); err == nil {
		t.Fatal("copy onto an existing file should fail")
	}
}

func TestRenameCopyRespectReadOnly(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t, WithReadOnly(true))
	if _, err := m.RenameFile(ctx, "a.http", "b.http"); err == nil {
		t.Fatal("rename should be blocked in read-only mode")
	}
	if _, err := m.CopyFile(ctx, "a.http", "b.http"); err == nil {
		t.Fatal("copy should be blocked in read-only mode")
	}
}
