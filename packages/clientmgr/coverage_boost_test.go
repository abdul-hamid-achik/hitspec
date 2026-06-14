package clientmgr

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestExportSnippetFormats(t *testing.T) {
	ctx := context.Background()
	srv := okServer(t)
	m := newTestManager(t)
	writeRunnableFile(t, m, "api.http", srv.URL)

	for _, format := range []string{"httpie", "python", "fetch", "go"} {
		out, err := m.Export(ctx, ExportReq{File: "api.http", RequestName: "ping", Format: format})
		if err != nil {
			t.Fatalf("Export(%s): %v", format, err)
		}
		if len(out.Commands) == 0 || out.Commands[0] == "" {
			t.Fatalf("Export(%s) produced no snippet", format)
		}
	}

	// A non-existent request name is an error.
	if _, err := m.Export(ctx, ExportReq{File: "api.http", RequestName: "nope", Format: "curl"}); err == nil {
		t.Fatal("Export of a missing request should error")
	}
}

func TestManagerOptionSetters(t *testing.T) {
	dir := t.TempDir()
	m := New(
		WithWorkDir(dir),
		WithWatch(false),
		WithHistoryDBPath(filepath.Join(dir, "history.db")),
		WithEnv("prod"),
		WithConfigPath(""),
		WithVerbose(true),
		WithAllowShell(true),
		WithAllowDB(true),
		WithLogFormat("json"),
		WithLogLevel("debug"),
	)
	m.Start(context.Background())
	t.Cleanup(func() { _ = m.Close() })

	if m.config.Env != "prod" {
		t.Fatalf("env = %q, want prod", m.config.Env)
	}
	if !m.config.Verbose || !m.config.AllowShell || !m.config.AllowDB {
		t.Fatalf("bool options not applied: %+v", m.config)
	}
	if m.config.LogFormat != "json" || m.config.LogLevel != "debug" {
		t.Fatalf("log options not applied: %+v", m.config)
	}
}

func TestCaptureCookiesFromResponse(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // isolate the cookie store
	ctx := context.Background()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "session", Value: "xyz", Path: "/"})
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	m := newTestManager(t)
	writeRunnableFile(t, m, "api.http", srv.URL)
	if _, err := m.RunFile(ctx, RunReq{File: "api.http"}); err != nil {
		t.Fatalf("RunFile: %v", err)
	}

	cookies, err := m.ListCookies(ctx)
	if err != nil {
		t.Fatalf("ListCookies: %v", err)
	}
	found := false
	for _, c := range cookies {
		if c.Name == "session" && c.Value == "xyz" {
			found = true
		}
	}
	if !found {
		t.Fatalf("Set-Cookie not captured into the store: %+v", cookies)
	}
}

func TestConfigGetter(t *testing.T) {
	dir := t.TempDir()
	m := New(WithWorkDir(dir), WithWatch(false), WithHistoryDBPath(filepath.Join(dir, "history.db")), WithEnv("dev"))
	m.Start(context.Background())
	t.Cleanup(func() { _ = m.Close() })
	if got := m.Config(); got.WorkDir != dir || got.Env != "dev" {
		t.Fatalf("Config() = %+v, want workdir %q env dev", got, dir)
	}
}

func TestWatcherEmitsFileEvent(t *testing.T) {
	dir := t.TempDir()
	m := New(
		WithWorkDir(dir),
		WithWatch(true),
		WithHistoryDBPath(filepath.Join(dir, "history.db")),
	)
	ctx := context.Background()
	events := m.Subscribe(ctx)
	m.Start(ctx)
	t.Cleanup(func() { _ = m.Close() })

	// Write a new .http file directly (not via CreateFile, which suppresses the
	// watcher) and expect a file event to be published.
	if err := os.WriteFile(filepath.Join(dir, "watched.http"), []byte("### r\nGET https://example.com\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	select {
	case ev := <-events:
		if ev.Type == "" {
			t.Fatalf("received empty event: %+v", ev)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for a watcher file event")
	}
}
