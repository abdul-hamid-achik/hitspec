package clientmgr

import (
	"context"
	"testing"
)

func TestCookiesCRUD(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)

	if got, err := m.ListCookies(ctx); err != nil || len(got) != 0 {
		t.Fatalf("initial cookies = %v, %v; want empty", got, err)
	}

	after, err := m.PutCookie(ctx, CookieDTO{Domain: "example.com", Path: "/", Name: "sid", Value: "abc"})
	if err != nil {
		t.Fatalf("PutCookie: %v", err)
	}
	if len(after) != 1 || after[0].Name != "sid" || after[0].Value != "abc" {
		t.Fatalf("after PutCookie = %+v, want one sid cookie", after)
	}

	list, err := m.ListCookies(ctx)
	if err != nil || len(list) != 1 {
		t.Fatalf("ListCookies = %v, %v; want one", list, err)
	}

	left, err := m.DeleteCookie(ctx, "example.com", "/", "sid")
	if err != nil {
		t.Fatalf("DeleteCookie: %v", err)
	}
	if len(left) != 0 {
		t.Fatalf("after DeleteCookie = %+v, want empty", left)
	}
}

func TestEnvironmentCRUDAndSelect(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)

	if _, err := m.PutEnvironment(ctx, "staging", map[string]any{"baseUrl": "https://staging.example.com"}); err != nil {
		t.Fatalf("PutEnvironment: %v", err)
	}

	envs, err := m.ListEnvironments(ctx)
	if err != nil {
		t.Fatalf("ListEnvironments: %v", err)
	}
	found := false
	for _, e := range envs {
		if e.Name == "staging" {
			found = true
		}
	}
	if !found {
		t.Fatalf("staging not in %+v", envs)
	}

	got, err := m.GetEnvironment(ctx, "staging")
	if err != nil {
		t.Fatalf("GetEnvironment: %v", err)
	}
	if got.Variables["baseUrl"] != "https://staging.example.com" {
		t.Fatalf("staging vars = %+v", got.Variables)
	}

	if err := m.SelectEnvironment(ctx, "staging"); err != nil {
		t.Fatalf("SelectEnvironment: %v", err)
	}
}

func TestConfigGetPut(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)

	if _, err := m.PutConfig(ctx, ConfigDTO{DefaultEnvironment: "dev", Timeout: 12000, Retries: 3, Concurrency: 4}); err != nil {
		t.Fatalf("PutConfig: %v", err)
	}
	cfg, err := m.GetConfig(ctx)
	if err != nil {
		t.Fatalf("GetConfig: %v", err)
	}
	if cfg.Timeout != 12000 || cfg.Retries != 3 || cfg.Concurrency != 4 {
		t.Fatalf("config not persisted: %+v", cfg)
	}
}

func TestListFilesAndGetFile(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	if _, err := m.CreateFile(ctx, "a/req.http", "### one\nGET https://example.com\n"); err != nil {
		t.Fatalf("CreateFile: %v", err)
	}

	files, err := m.ListFiles(ctx)
	if err != nil || len(files) != 1 {
		t.Fatalf("ListFiles = %v, %v; want one", files, err)
	}
	if files[0].RelativePath != "a/req.http" {
		t.Fatalf("relpath = %q", files[0].RelativePath)
	}

	parsed, err := m.GetFile(ctx, "a/req.http")
	if err != nil {
		t.Fatalf("GetFile: %v", err)
	}
	if len(parsed.Requests) != 1 {
		t.Fatalf("parsed requests = %d, want 1", len(parsed.Requests))
	}
}

func TestSystemInfo(t *testing.T) {
	m := newTestManager(t)
	info := m.SystemInfo()
	if info.GoVersion == "" || info.OS == "" || info.Arch == "" {
		t.Fatalf("incomplete system info: %+v", info)
	}
}

func TestInMemoryHistory(t *testing.T) {
	ctx := context.Background()
	srv := okServer(t)
	m := newTestManager(t)
	writeRunnableFile(t, m, "api.http", srv.URL)
	if _, err := m.RunFile(ctx, RunReq{File: "api.http"}); err != nil {
		t.Fatalf("RunFile: %v", err)
	}
	entries, err := m.InMemoryHistory(ctx)
	if err != nil {
		t.Fatalf("InMemoryHistory: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected in-memory history entries after a run")
	}
	if err := m.ClearInMemoryHistory(ctx); err != nil {
		t.Fatalf("ClearInMemoryHistory: %v", err)
	}
	if entries, _ := m.InMemoryHistory(ctx); len(entries) != 0 {
		t.Fatalf("history not cleared: %+v", entries)
	}
}
