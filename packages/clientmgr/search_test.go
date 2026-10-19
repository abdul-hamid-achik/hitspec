package clientmgr

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/abdul-hamid-achik/hitspec/v2/packages/core/parser"
)

func TestSearchRequests(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	if _, err := m.CreateFile(ctx, "users.http", "### List users\n# @tags smoke\nGET https://api.example.com/users\n\n### Create user\nPOST https://api.example.com/users\n"); err != nil {
		t.Fatalf("create users: %v", err)
	}
	if _, err := m.CreateFile(ctx, "auth.http", "### Login\nPOST https://api.example.com/login\n"); err != nil {
		t.Fatalf("create auth: %v", err)
	}

	// Match by request name.
	if res, _ := m.SearchRequests(ctx, "login"); len(res) != 1 || res[0].RequestName != "Login" {
		t.Fatalf("search 'login' -> %+v", res)
	}
	// Match by URL substring (both /users requests).
	if res, _ := m.SearchRequests(ctx, "users"); len(res) != 2 {
		t.Fatalf("search 'users' -> want 2, got %d", len(res))
	}
	// Match by tag.
	if res, _ := m.SearchRequests(ctx, "smoke"); len(res) != 1 {
		t.Fatalf("search 'smoke' -> want 1, got %d", len(res))
	}
	// Case-insensitive.
	if res, _ := m.SearchRequests(ctx, "LOGIN"); len(res) != 1 {
		t.Fatalf("search 'LOGIN' should be case-insensitive, got %d", len(res))
	}
	// Empty query returns nothing.
	if res, _ := m.SearchRequests(ctx, "   "); res != nil {
		t.Fatalf("empty query should return nil, got %+v", res)
	}
}

// countParses swaps the parse function for one that counts calls per path.
func countParses(t *testing.T) func() int {
	t.Helper()
	var mu sync.Mutex
	n := 0
	orig := parseFile
	parseFile = func(path string) (*parser.File, error) {
		mu.Lock()
		n++
		mu.Unlock()
		return orig(path)
	}
	t.Cleanup(func() { parseFile = orig })
	return func() int {
		mu.Lock()
		defer mu.Unlock()
		return n
	}
}

func searchNames(t *testing.T, m *Manager, q string) []string {
	t.Helper()
	res, err := m.SearchRequests(context.Background(), q)
	if err != nil {
		t.Fatalf("search %q: %v", q, err)
	}
	var names []string
	for _, r := range res {
		names = append(names, r.RequestName)
	}
	return names
}

func TestSearchCacheSkipsUnchangedFiles(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	for name, body := range map[string]string{
		"a.http": "### Alpha\nGET https://x/alpha\n",
		"b.http": "### Beta\nGET https://x/beta\n",
	} {
		if _, err := m.CreateFile(ctx, name, body); err != nil {
			t.Fatal(err)
		}
	}
	parses := countParses(t)
	before := parses()
	if got := searchNames(t, m, "alpha"); len(got) != 1 {
		t.Fatalf("alpha -> %v", got)
	}
	if got := parses() - before; got != 2 {
		t.Fatalf("first search parsed %d files, want 2", got)
	}
	// More keystrokes over unchanged files parse nothing.
	for _, q := range []string{"alph", "al", "beta", "x"} {
		searchNames(t, m, q)
	}
	if got := parses() - before; got != 2 {
		t.Fatalf("later searches re-parsed: %d parses in total, want 2", got)
	}
}

func TestSearchCacheReparsesAfterSaveFile(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	if _, err := m.CreateFile(ctx, "a.http", "### Alpha\nGET https://x/alpha\n"); err != nil {
		t.Fatal(err)
	}
	searchNames(t, m, "alpha") // warm the cache
	parses := countParses(t)

	// Same size, and the mtime is pinned to the old one below: only the explicit
	// invalidation in SaveFile can notice this edit.
	abs := filepath.Join(m.config.WorkDir, "a.http")
	info, err := os.Stat(abs)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.SaveFile(ctx, "a.http", "### Omega\nGET https://x/alpha\n"); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(abs, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	// SaveFile re-parses the saved file itself; count only what search does.
	base := parses()
	if got := searchNames(t, m, "omega"); len(got) != 1 || got[0] != "Omega" {
		t.Fatalf("saved edit not found: %v", got)
	}
	if got := parses() - base; got != 1 {
		t.Fatalf("search parsed %d files after SaveFile, want 1", got)
	}
	if got := searchNames(t, m, "alpha"); len(got) != 1 || got[0] != "Omega" {
		t.Fatalf("the url still matches the new name: %v", got)
	}
}

func TestSearchCacheReparsesExternalEditThatChangesSize(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	if _, err := m.CreateFile(ctx, "a.http", "### Alpha\nGET https://x/alpha\n"); err != nil {
		t.Fatal(err)
	}
	searchNames(t, m, "alpha")
	abs := filepath.Join(m.config.WorkDir, "a.http")
	info, err := os.Stat(abs)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(abs, []byte("### Alpha\nGET https://x/alpha\n\n### Gamma\nGET https://x/gamma\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Keep the old mtime so only the size differs.
	if err := os.Chtimes(abs, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	if got := searchNames(t, m, "gamma"); len(got) != 1 {
		t.Fatalf("external edit not seen: %v", got)
	}
}

func TestSearchCacheReparsesExternalEditThatChangesMtime(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	if _, err := m.CreateFile(ctx, "a.http", "### Alpha\nGET https://x/alpha\n"); err != nil {
		t.Fatal(err)
	}
	searchNames(t, m, "alpha")
	abs := filepath.Join(m.config.WorkDir, "a.http")
	// Same size ("Alpha" -> "Gamma"), later mtime.
	if err := os.WriteFile(abs, []byte("### Gamma\nGET https://x/alpha\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	later := time.Now().Add(time.Hour)
	if err := os.Chtimes(abs, later, later); err != nil {
		t.Fatal(err)
	}
	if got := searchNames(t, m, "gamma"); len(got) != 1 {
		t.Fatalf("mtime change not seen: %v", got)
	}
}

func TestSearchCacheDropsDeletedAndRenamedFiles(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	for name, body := range map[string]string{
		"a.http": "### Alpha\nGET https://x/alpha\n",
		"b.http": "### Beta\nGET https://x/beta\n",
		"c.http": "### Cee\nGET https://x/cee\n",
	} {
		if _, err := m.CreateFile(ctx, name, body); err != nil {
			t.Fatal(err)
		}
	}
	searchNames(t, m, "x/") // warm all three

	// Deleted on disk behind the manager's back.
	if err := os.Remove(filepath.Join(m.config.WorkDir, "a.http")); err != nil {
		t.Fatal(err)
	}
	if got := searchNames(t, m, "alpha"); len(got) != 0 {
		t.Fatalf("deleted file still found: %v", got)
	}
	if n := len(m.search.files); n != 2 {
		t.Fatalf("cache holds %d entries after the delete, want 2", n)
	}

	if err := m.DeleteFile(ctx, "b.http"); err != nil {
		t.Fatal(err)
	}
	if got := searchNames(t, m, "beta"); len(got) != 0 {
		t.Fatalf("file deleted through the manager still found: %v", got)
	}

	if _, err := m.RenameFile(ctx, "c.http", "d.http"); err != nil {
		t.Fatal(err)
	}
	res, err := m.SearchRequests(ctx, "cee")
	if err != nil || len(res) != 1 || res[0].File != "d.http" {
		t.Fatalf("renamed file: %+v, %v", res, err)
	}
}

func TestSearchRequestsHonorsCancelledContext(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	if _, err := m.CreateFile(ctx, "a.http", "### Alpha\nGET https://x/alpha\n"); err != nil {
		t.Fatal(err)
	}
	cctx, cancel := context.WithCancel(ctx)
	cancel()
	res, err := m.SearchRequests(cctx, "alpha")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if res != nil {
		t.Fatalf("results with an error: %+v", res)
	}
}

// Cancelling in the middle of the walk stops it between files.
func TestSearchRequestsStopsBetweenFiles(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	for _, name := range []string{"a.http", "b.http", "c.http"} {
		if _, err := m.CreateFile(ctx, name, "### R\nGET https://x/r\n"); err != nil {
			t.Fatal(err)
		}
	}
	cctx, cancel := context.WithCancel(ctx)
	orig := parseFile
	n := 0
	parseFile = func(path string) (*parser.File, error) {
		n++
		cancel() // cancelled while the first file is being parsed
		return orig(path)
	}
	t.Cleanup(func() { parseFile = orig })
	if _, err := m.SearchRequests(cctx, "r"); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if n != 1 {
		t.Fatalf("parsed %d files after cancellation, want 1", n)
	}
}

func TestSearchCacheResultsAreNotShared(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	if _, err := m.CreateFile(ctx, "a.http", "### Alpha\n# @tags smoke, api\nGET https://x/alpha\n"); err != nil {
		t.Fatal(err)
	}
	res, _ := m.SearchRequests(ctx, "alpha")
	if len(res) != 1 || len(res[0].Tags) == 0 {
		t.Fatalf("res = %+v", res)
	}
	res[0].Tags[0] = "mutated"
	again, _ := m.SearchRequests(ctx, "alpha")
	if again[0].Tags[0] == "mutated" {
		t.Fatal("a caller mutating the tags changed the cached entry")
	}
}
