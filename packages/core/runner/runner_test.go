package runner

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/abdul-hamid-achik/hitspec/packages/core/parser"
	"github.com/abdul-hamid-achik/hitspec/packages/history"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRunner(t *testing.T) {
	t.Run("with nil config", func(t *testing.T) {
		r := NewRunner(nil)
		assert.NotNil(t, r)
		assert.NotNil(t, r.client)
		assert.NotNil(t, r.resolver)
	})

	t.Run("with custom config", func(t *testing.T) {
		cfg := &Config{
			Environment: "test",
			Verbose:     true,
			Parallel:    true,
			Concurrency: 10,
		}
		r := NewRunner(cfg)
		assert.NotNil(t, r)
		assert.Equal(t, "test", r.config.Environment)
		assert.True(t, r.config.Verbose)
	})
}

func TestRunner_RunFile(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status": "ok", "items": [1, 2, 3]}`))
	}))
	defer server.Close()

	// Create a temporary test file
	content := `### Test Request
GET ` + server.URL + `/test

>>>
expect status 200
expect body.status == "ok"
expect body.items length 3
<<<`

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.http")
	err := os.WriteFile(testFile, []byte(content), 0644)
	require.NoError(t, err)

	// Run the test
	r := NewRunner(&Config{})
	result, err := r.RunFile(testFile)

	require.NoError(t, err)
	assert.Equal(t, 1, result.Passed)
	assert.Equal(t, 0, result.Failed)
	assert.Len(t, result.Results, 1)
	assert.True(t, result.Results[0].Passed)
}

func TestRunner_RunFile_WithFailingAssertion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	content := `### Test Request
GET ` + server.URL + `/test

>>>
expect status 200
<<<`

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.http")
	err := os.WriteFile(testFile, []byte(content), 0644)
	require.NoError(t, err)

	r := NewRunner(&Config{})
	result, err := r.RunFile(testFile)

	require.NoError(t, err)
	assert.Equal(t, 0, result.Passed)
	assert.Equal(t, 1, result.Failed)
	assert.False(t, result.Results[0].Passed)
}

func TestRunner_RunFile_WithSkip(t *testing.T) {
	content := `### Skipped Test
# @skip This test is skipped

GET http://example.com/test`

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.http")
	err := os.WriteFile(testFile, []byte(content), 0644)
	require.NoError(t, err)

	r := NewRunner(&Config{})
	result, err := r.RunFile(testFile)

	require.NoError(t, err)
	assert.Equal(t, 0, result.Passed)
	assert.Equal(t, 0, result.Failed)
	assert.Equal(t, 1, result.Skipped)
	assert.True(t, result.Results[0].Skipped)
}

func TestRunner_TopologicalSort_CircularDependency(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	content := `### Request A
# @name requestA
# @depends requestB

GET ` + server.URL + `/a

### Request B
# @name requestB
# @depends requestA

GET ` + server.URL + `/b`

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.http")
	err := os.WriteFile(testFile, []byte(content), 0644)
	require.NoError(t, err)

	r := NewRunner(&Config{})
	_, err = r.RunFile(testFile)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "circular dependency")
}

// TestRunner_TopologicalSort_DuplicateNames guards the regression where two
// requests sharing an @name collided in the name-keyed graph maps, producing
// either a bogus "circular dependency detected involving requests: []" abort
// or silently dropping one of the requests. Duplicate names must be reported
// clearly so captures and @depends resolve deterministically.
func TestRunner_TopologicalSort_DuplicateNames(t *testing.T) {
	content := `### First
# @name getUser

GET http://example.com/first

### Second
# @name getUser

GET http://example.com/second
`
	file, err := parser.Parse(content, "dup.http")
	require.NoError(t, err)
	require.Len(t, file.Requests, 2)

	r := NewRunner(&Config{})
	_, err = r.topologicalSort(file.Requests)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate request name")
	assert.Contains(t, err.Error(), "getUser")
}

// TestRunner_TopologicalSort_AnonymousCoexist ensures unnamed requests (which
// get synthetic names) still sort without being mistaken for duplicates.
func TestRunner_TopologicalSort_AnonymousCoexist(t *testing.T) {
	content := `### First
GET http://example.com/first

### Second
GET http://example.com/second
`
	file, err := parser.Parse(content, "anon.http")
	require.NoError(t, err)
	require.Len(t, file.Requests, 2)

	r := NewRunner(&Config{})
	sorted, err := r.topologicalSort(file.Requests)
	require.NoError(t, err)
	require.Len(t, sorted, 2)
}

// TestRunner_IndexFilter_Untitled guards the regression where the studio TUI
// had no way to run a single untitled request: passing the synthesized "line N"
// display name as NameFilter matched nothing (req.Name is "" for untitled
// requests) and the run was a silent no-op. IndexFilter selects the request by
// its source position instead.
func TestRunner_IndexFilter_Untitled(t *testing.T) {
	var hit []string
	var mu sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		hit = append(hit, r.URL.Path)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Two untitled requests to distinct paths; the NameFilter ("") would run
	// both, IndexFilter must run only the second.
	content := fmt.Sprintf("### First\nGET %s/first\n\n### Second\nGET %s/second\n", server.URL, server.URL)
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "t.http")
	require.NoError(t, os.WriteFile(testFile, []byte(content), 0o644))

	idx := 1
	r := NewRunner(&Config{IndexFilter: &idx})
	result, err := r.RunFile(testFile)
	require.NoError(t, err)
	require.Len(t, result.Results, 1)
	assert.True(t, result.Results[0].Passed)
	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, []string{"/second"}, hit, "IndexFilter must run only the selected request")
}

func TestRunner_DependencyOrder(t *testing.T) {
	executionOrder := []string{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		executionOrder = append(executionOrder, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"token": "abc123"}`))
	}))
	defer server.Close()

	content := `### Request B
# @name requestB
# @depends requestA

GET ` + server.URL + `/b

### Request A
# @name requestA

GET ` + server.URL + `/a`

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.http")
	err := os.WriteFile(testFile, []byte(content), 0644)
	require.NoError(t, err)

	r := NewRunner(&Config{})
	result, err := r.RunFile(testFile)

	require.NoError(t, err)
	assert.Equal(t, 2, result.Passed)
	// Request A should be executed before Request B due to dependency
	assert.Equal(t, []string{"/a", "/b"}, executionOrder)
}

// TestRunner_DependencyOnFilteredRequestSkips guards the regression where a
// request whose @depends target was filtered out (or skipped/nonexistent) ran
// anyway because the dependency check only flagged deps that ran and failed.
// The dependent must be skipped when its dependency didn't run.
func TestRunner_DependencyOnFilteredRequestSkipsDependent(t *testing.T) {
	var hit []string
	var mu sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		hit = append(hit, r.URL.Path)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// A (name a) -> B (name b, depends on a). Run with --name b so a is filtered
	// out; B must be skipped because its dependency didn't run.
	content := "### A\n# @name a\nGET " + server.URL + "/a\n\n### B\n# @name b\n# @depends a\nGET " + server.URL + "/b\n"
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.http")
	require.NoError(t, os.WriteFile(testFile, []byte(content), 0o644))

	r := NewRunner(&Config{NameFilter: "b"})
	result, err := r.RunFile(testFile)
	require.NoError(t, err)
	// A is filtered out ("filtered out" stub) and B is skipped because its
	// dependency a didn't run; nothing executes against the server.
	assert.Equal(t, 0, result.Passed)
	assert.Equal(t, 2, result.Skipped)
	mu.Lock()
	defer mu.Unlock()
	assert.Empty(t, hit, "dependent must not run when its dependency was filtered out")
}

// TestRunner_WaitHistoryFlushesBeforeClose guards the close/write race that
// lost history rows: recordHistory runs in a goroutine, and the caller's
// deferred store.Close() used to fire while it was still writing. WaitHistory
// must block until the row is persisted, so a subsequent Close is safe.
func TestRunner_WaitHistoryFlushesBeforeClose(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	dir := t.TempDir()
	store, err := history.NewStore(filepath.Join(dir, "history.db"))
	require.NoError(t, err)

	httpFile := filepath.Join(dir, "ok.http")
	require.NoError(t, os.WriteFile(httpFile, []byte("### ok\nGET "+server.URL+"/ok\n"), 0o644))

	r := NewRunner(&Config{HistoryStore: store})
	_, err = r.RunFile(httpFile)
	require.NoError(t, err)

	// Flush the background history writer before closing the store.
	r.WaitHistory()
	require.NoError(t, store.Close())

	// Reopen and confirm the run was persisted (not lost to the close race).
	store2, err := history.NewStore(filepath.Join(dir, "history.db"))
	require.NoError(t, err)
	defer store2.Close()
	count, err := store2.Queries().CountRuns(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int64(1), count, "history run row must be persisted after WaitHistory+Close")
}

// TestRunner_RunParallel exercises the runParallel concurrency path (0%
// coverage) under the race detector. Multiple independent requests run
// concurrently with a bounded concurrency; all should pass and the OnProgress
// callback must be safe to invoke from concurrent goroutines.
func TestRunner_RunParallel(t *testing.T) {
	var mu sync.Mutex
	var progressCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// 6 independent requests to the same server.
	content := ""
	for i := 0; i < 6; i++ {
		content += fmt.Sprintf("### req%d\nGET %s/%d\n\n", i, server.URL, i)
	}
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "par.http")
	require.NoError(t, os.WriteFile(testFile, []byte(content), 0o644))

	r := NewRunner(&Config{
		Parallel:    true,
		Concurrency: 3,
		OnProgress: func(event ProgressEvent) {
			mu.Lock()
			progressCount++
			mu.Unlock()
		},
	})
	result, err := r.RunFile(testFile)
	require.NoError(t, err)
	assert.Equal(t, 6, result.Passed, "all parallel requests should pass")
	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, 12, progressCount, "OnProgress should fire started+completed per request")
}

// TestRunner_DBAssertionsRequireAllowDB covers the --allow-db gate (0%
// coverage): executeDBAssertions must refuse to run without AllowDB set, even
// when a connection string and assertions are present.
func TestRunner_DBAssertionsRequireAllowDB(t *testing.T) {
	r := NewRunner(&Config{AllowDB: false})
	_, err := r.executeDBAssertions(
		[]*parser.DBAssertion{{Query: "SELECT 1", Column: "1", Operator: parser.OpEquals, Expected: 1}},
		"sqlite3://"+filepath.Join(t.TempDir(), "x.db"),
		func(s string) string { return s },
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "allow-db")

	// With AllowDB true against a fresh sqlite DB, a trivial assertion runs.
	r2 := NewRunner(&Config{AllowDB: true})
	dbPath := filepath.Join(t.TempDir(), "gate.db")
	// Initialize the sqlite file so the connection succeeds.
	store, serr := history.NewStore(dbPath)
	require.NoError(t, serr)
	store.Close()
	results, err := r2.executeDBAssertions(
		[]*parser.DBAssertion{{Query: "SELECT 1 AS one", Column: "one", Operator: parser.OpEquals, Expected: 1}},
		"sqlite://"+dbPath,
		func(s string) string { return s },
	)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.True(t, results[0].Passed, "SELECT 1 AS one == 1 should pass; message: %s", results[0].Message)
}

// TestRunner_TopologicalSort_SourceOrder guards the regression where
// topologicalSort seeded Kahn's ready queue from Go map iteration (randomised
// order), so requests without @depends ran in nondeterministic order. The
// quickstart promises "executes each request in order"; independent requests
// must therefore preserve source (file) order. Run many iterations to catch
// the map-iteration nondeterminism the old code relied on.
func TestRunner_TopologicalSort_SourceOrder(t *testing.T) {
	content := `### First
# @name first

GET http://example.com/first

### Second
# @name second

GET http://example.com/second

### Third
# @name third

GET http://example.com/third

### Fourth
# @name fourth

GET http://example.com/fourth
`
	file, err := parser.Parse(content, "order.http")
	require.NoError(t, err)
	require.Len(t, file.Requests, 4)

	r := NewRunner(&Config{})
	want := []string{"first", "second", "third", "fourth"}
	for i := 0; i < 100; i++ {
		sorted, err := r.topologicalSort(file.Requests)
		require.NoError(t, err)
		require.Len(t, sorted, 4)
		var got []string
		for _, req := range sorted {
			got = append(got, req.Name)
		}
		assert.Equal(t, want, got, "iteration %d: independent requests must run in source order", i)
	}
}

func TestRunner_NameFilter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	content := `### First Request
# @name first

GET ` + server.URL + `/first

### Second Request
# @name second

GET ` + server.URL + `/second`

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.http")
	err := os.WriteFile(testFile, []byte(content), 0644)
	require.NoError(t, err)

	r := NewRunner(&Config{NameFilter: "first"})
	result, err := r.RunFile(testFile)

	require.NoError(t, err)
	assert.Equal(t, 1, result.Passed)
	assert.Equal(t, 1, result.Skipped) // second request filtered out
}

func TestRunner_TagsFilter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	content := `### Smoke Test
# @tags smoke, api

GET ` + server.URL + `/smoke

### Integration Test
# @tags integration

GET ` + server.URL + `/integration`

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.http")
	err := os.WriteFile(testFile, []byte(content), 0644)
	require.NoError(t, err)

	r := NewRunner(&Config{TagsFilter: []string{"smoke"}})
	result, err := r.RunFile(testFile)

	require.NoError(t, err)
	assert.Equal(t, 1, result.Passed)
	assert.Equal(t, 1, result.Skipped)
}

func TestRunner_Bail(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	content := `### First Request
GET ` + server.URL + `/first

>>>
expect status 200
<<<

### Second Request
GET ` + server.URL + `/second

>>>
expect status 200
<<<`

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.http")
	err := os.WriteFile(testFile, []byte(content), 0644)
	require.NoError(t, err)

	r := NewRunner(&Config{Bail: true})
	result, err := r.RunFile(testFile)

	require.NoError(t, err)
	assert.Equal(t, 1, result.Failed)
	assert.Equal(t, 1, requestCount) // Should stop after first failure
}

func TestMatchesPattern(t *testing.T) {
	tests := []struct {
		name     string
		pattern  string
		expected bool
	}{
		{"exact match", "testName", true},
		{"prefix match", "test*", true},
		{"suffix match", "*Name", true},
		{"contains match", "*stNa*", true},
		{"no match", "other*", false},
		{"empty pattern", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name+" - "+tt.pattern, func(t *testing.T) {
			result := matchesPattern("testName", tt.pattern)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestHasAnyTag(t *testing.T) {
	tests := []struct {
		tags     []string
		filters  []string
		expected bool
	}{
		{[]string{"smoke", "api"}, []string{"smoke"}, true},
		{[]string{"smoke", "api"}, []string{"integration"}, false},
		{[]string{"smoke", "api"}, []string{"smoke", "integration"}, true},
		{[]string{}, []string{"smoke"}, false},
		{[]string{"smoke"}, []string{}, false},
	}

	for _, tt := range tests {
		result := parser.HasAnyTag(tt.tags, tt.filters)
		assert.Equal(t, tt.expected, result)
	}
}

func TestRunner_Shell(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	t.Run("executes shell commands after request", func(t *testing.T) {
		tmpDir := t.TempDir()
		markerFile := filepath.Join(tmpDir, "shell_marker.txt")

		content := `### Test with shell commands
GET ` + server.URL + `/test

>>>
expect status 200
<<<

>>>shell
echo "executed" > ` + markerFile + `
<<<`

		testFile := filepath.Join(tmpDir, "test.http")
		err := os.WriteFile(testFile, []byte(content), 0644)
		require.NoError(t, err)

		r := NewRunner(&Config{AllowShell: true})
		result, err := r.RunFile(testFile)

		require.NoError(t, err)
		assert.True(t, result.Results[0].Passed)

		// Verify shell command was executed
		data, err := os.ReadFile(markerFile)
		require.NoError(t, err)
		assert.Contains(t, string(data), "executed")
	})

	t.Run("fails on shell command error", func(t *testing.T) {
		tmpDir := t.TempDir()

		content := `### Test with failing shell
GET ` + server.URL + `/test

>>>
expect status 200
<<<

>>>shell
exit 1
<<<`

		testFile := filepath.Join(tmpDir, "test.http")
		err := os.WriteFile(testFile, []byte(content), 0644)
		require.NoError(t, err)

		r := NewRunner(&Config{AllowShell: true})
		result, err := r.RunFile(testFile)

		require.NoError(t, err)
		assert.False(t, result.Results[0].Passed)
		assert.NotNil(t, result.Results[0].Error)
	})

	t.Run("ignores errors with dash prefix", func(t *testing.T) {
		tmpDir := t.TempDir()

		content := `### Test with ignored error
GET ` + server.URL + `/test

>>>
expect status 200
<<<

>>>shell
-exit 1
<<<`

		testFile := filepath.Join(tmpDir, "test.http")
		err := os.WriteFile(testFile, []byte(content), 0644)
		require.NoError(t, err)

		r := NewRunner(&Config{AllowShell: true})
		result, err := r.RunFile(testFile)

		require.NoError(t, err)
		assert.True(t, result.Results[0].Passed)
	})

	t.Run("resolves variables in commands", func(t *testing.T) {
		tmpDir := t.TempDir()
		markerFile := filepath.Join(tmpDir, "var_marker.txt")

		content := `@testValue = hello

### Test with variables
GET ` + server.URL + `/test

>>>
expect status 200
<<<

>>>shell
echo "{{testValue}}" > ` + markerFile + `
<<<`

		testFile := filepath.Join(tmpDir, "test.http")
		err := os.WriteFile(testFile, []byte(content), 0644)
		require.NoError(t, err)

		r := NewRunner(&Config{AllowShell: true})
		result, err := r.RunFile(testFile)

		require.NoError(t, err)
		assert.True(t, result.Results[0].Passed)

		data, err := os.ReadFile(markerFile)
		require.NoError(t, err)
		assert.Contains(t, string(data), "hello")
	})
}

func TestRunner_WaitFor(t *testing.T) {
	t.Run("waits for service to be ready", func(t *testing.T) {
		requestCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestCount++
			if r.URL.Path == "/health" {
				w.WriteHeader(http.StatusOK)
				return
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		content := `### Wait for health check
# @waitFor ` + server.URL + `/health 200 5000 100

GET ` + server.URL + `/api

>>>
expect status 200
<<<`

		tmpDir := t.TempDir()
		testFile := filepath.Join(tmpDir, "test.http")
		err := os.WriteFile(testFile, []byte(content), 0644)
		require.NoError(t, err)

		r := NewRunner(&Config{})
		result, err := r.RunFile(testFile)

		require.NoError(t, err)
		assert.True(t, result.Results[0].Passed)
		assert.GreaterOrEqual(t, requestCount, 2) // At least health check + actual request
	})

	t.Run("times out if service not ready", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer server.Close()

		content := `### Wait for service that never becomes ready
# @waitFor ` + server.URL + `/health 200 500 100

GET ` + server.URL + `/api

>>>
expect status 200
<<<`

		tmpDir := t.TempDir()
		testFile := filepath.Join(tmpDir, "test.http")
		err := os.WriteFile(testFile, []byte(content), 0644)
		require.NoError(t, err)

		r := NewRunner(&Config{})
		result, err := r.RunFile(testFile)

		require.NoError(t, err)
		assert.False(t, result.Results[0].Passed)
		assert.Contains(t, result.Results[0].Error.Error(), "not ready")
	})
}

func TestRunner_Hooks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	t.Run("before hook runs before request", func(t *testing.T) {
		tmpDir := t.TempDir()
		markerFile := filepath.Join(tmpDir, "before_marker.txt")

		// Create a setup script
		setupScript := filepath.Join(tmpDir, "setup.sh")
		err := os.WriteFile(setupScript, []byte("#!/bin/bash\necho 'before' > "+markerFile), 0755)
		require.NoError(t, err)

		content := `### Test with before hook
# @before ./setup.sh

GET ` + server.URL + `/test

>>>
expect status 200
<<<`

		testFile := filepath.Join(tmpDir, "test.http")
		err = os.WriteFile(testFile, []byte(content), 0644)
		require.NoError(t, err)

		r := NewRunner(&Config{AllowShell: true})
		result, err := r.RunFile(testFile)

		require.NoError(t, err)
		assert.True(t, result.Results[0].Passed)

		// Verify before hook was executed
		data, err := os.ReadFile(markerFile)
		require.NoError(t, err)
		assert.Contains(t, string(data), "before")
	})

	t.Run("after hook runs after request", func(t *testing.T) {
		tmpDir := t.TempDir()
		markerFile := filepath.Join(tmpDir, "after_marker.txt")

		// Create a cleanup script
		cleanupScript := filepath.Join(tmpDir, "cleanup.sh")
		err := os.WriteFile(cleanupScript, []byte("#!/bin/bash\necho 'after' > "+markerFile), 0755)
		require.NoError(t, err)

		content := `### Test with after hook
# @after ./cleanup.sh

GET ` + server.URL + `/test

>>>
expect status 200
<<<`

		testFile := filepath.Join(tmpDir, "test.http")
		err = os.WriteFile(testFile, []byte(content), 0644)
		require.NoError(t, err)

		r := NewRunner(&Config{AllowShell: true})
		result, err := r.RunFile(testFile)

		require.NoError(t, err)
		assert.True(t, result.Results[0].Passed)

		// Verify after hook was executed
		data, err := os.ReadFile(markerFile)
		require.NoError(t, err)
		assert.Contains(t, string(data), "after")
	})

	t.Run("after hook runs even on failed assertion", func(t *testing.T) {
		failServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer failServer.Close()

		tmpDir := t.TempDir()
		markerFile := filepath.Join(tmpDir, "cleanup_marker.txt")

		// Create a cleanup script
		cleanupScript := filepath.Join(tmpDir, "cleanup.sh")
		err := os.WriteFile(cleanupScript, []byte("#!/bin/bash\necho 'cleanup' > "+markerFile), 0755)
		require.NoError(t, err)

		content := `### Test with after hook on failure
# @after ./cleanup.sh

GET ` + failServer.URL + `/test

>>>
expect status 200
<<<`

		testFile := filepath.Join(tmpDir, "test.http")
		err = os.WriteFile(testFile, []byte(content), 0644)
		require.NoError(t, err)

		r := NewRunner(&Config{AllowShell: true})
		result, err := r.RunFile(testFile)

		require.NoError(t, err)
		assert.False(t, result.Results[0].Passed) // Request failed

		// Verify after hook was still executed
		data, err := os.ReadFile(markerFile)
		require.NoError(t, err)
		assert.Contains(t, string(data), "cleanup")
	})

	t.Run("multiple hooks execute in order", func(t *testing.T) {
		tmpDir := t.TempDir()
		orderFile := filepath.Join(tmpDir, "order.txt")

		// Create setup scripts
		setup1 := filepath.Join(tmpDir, "setup1.sh")
		err := os.WriteFile(setup1, []byte("#!/bin/bash\necho '1' >> "+orderFile), 0755)
		require.NoError(t, err)

		setup2 := filepath.Join(tmpDir, "setup2.sh")
		err = os.WriteFile(setup2, []byte("#!/bin/bash\necho '2' >> "+orderFile), 0755)
		require.NoError(t, err)

		content := `### Test with multiple hooks
# @before ./setup1.sh
# @before ./setup2.sh

GET ` + server.URL + `/test

>>>
expect status 200
<<<`

		testFile := filepath.Join(tmpDir, "test.http")
		err = os.WriteFile(testFile, []byte(content), 0644)
		require.NoError(t, err)

		r := NewRunner(&Config{AllowShell: true})
		result, err := r.RunFile(testFile)

		require.NoError(t, err)
		assert.True(t, result.Results[0].Passed)

		// Verify hooks executed in order
		data, err := os.ReadFile(orderFile)
		require.NoError(t, err)
		assert.Equal(t, "1\n2\n", string(data))
	})
}

func TestRunner_ConditionIf_Truthy(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	content := `@runTests = true

### Conditional Request
# @if {{runTests}}

GET ` + server.URL + `/test

>>>
expect status 200
<<<`

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.http")
	err := os.WriteFile(testFile, []byte(content), 0644)
	require.NoError(t, err)

	r := NewRunner(&Config{})
	result, err := r.RunFile(testFile)

	require.NoError(t, err)
	assert.Equal(t, 1, result.Passed)
	assert.Equal(t, 0, result.Skipped)
}

func TestRunner_ConditionIf_Falsy(t *testing.T) {
	content := `@runTests = false

### Conditional Request
# @if {{runTests}}

GET http://example.com/test

>>>
expect status 200
<<<`

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.http")
	err := os.WriteFile(testFile, []byte(content), 0644)
	require.NoError(t, err)

	r := NewRunner(&Config{})
	result, err := r.RunFile(testFile)

	require.NoError(t, err)
	assert.Equal(t, 0, result.Passed)
	assert.Equal(t, 1, result.Skipped)
	assert.Equal(t, "condition not met", result.Results[0].SkipReason)
}

func TestRunner_ConditionIf_UnresolvedVariable(t *testing.T) {
	// Unresolved variable references are falsy
	content := `### Conditional Request
# @if {{undefinedVar}}

GET http://example.com/test`

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.http")
	err := os.WriteFile(testFile, []byte(content), 0644)
	require.NoError(t, err)

	r := NewRunner(&Config{})
	result, err := r.RunFile(testFile)

	require.NoError(t, err)
	assert.Equal(t, 0, result.Passed)
	assert.Equal(t, 1, result.Skipped)
}

func TestRunner_ConditionUnless_Truthy(t *testing.T) {
	// @unless with truthy value should skip
	content := `@skipAuth = true

### Skip Auth Test
# @unless {{skipAuth}}

GET http://example.com/auth/test`

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.http")
	err := os.WriteFile(testFile, []byte(content), 0644)
	require.NoError(t, err)

	r := NewRunner(&Config{})
	result, err := r.RunFile(testFile)

	require.NoError(t, err)
	assert.Equal(t, 0, result.Passed)
	assert.Equal(t, 1, result.Skipped)
}

func TestRunner_ConditionUnless_Falsy(t *testing.T) {
	// @unless with falsy value should run
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	content := `@skipAuth = false

### Run Auth Test
# @unless {{skipAuth}}

GET ` + server.URL + `/auth/test

>>>
expect status 200
<<<`

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.http")
	err := os.WriteFile(testFile, []byte(content), 0644)
	require.NoError(t, err)

	r := NewRunner(&Config{})
	result, err := r.RunFile(testFile)

	require.NoError(t, err)
	assert.Equal(t, 1, result.Passed)
	assert.Equal(t, 0, result.Skipped)
}

func TestRunner_RetryOnStatusCode(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts <= 2 {
			w.WriteHeader(http.StatusServiceUnavailable) // 503
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	content := `### Retry On 503
# @retry 3
# @retryOn 503
# @retryDelay 10

GET ` + server.URL + `/test

>>>
expect status 200
<<<`

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.http")
	err := os.WriteFile(testFile, []byte(content), 0644)
	require.NoError(t, err)

	r := NewRunner(&Config{})
	result, err := r.RunFile(testFile)

	require.NoError(t, err)
	assert.Equal(t, 1, result.Passed)
	assert.Equal(t, 3, attempts) // 2 failures + 1 success
}

func TestIsTruthy(t *testing.T) {
	tests := []struct {
		value    string
		expected bool
	}{
		{"true", true},
		{"false", false},
		{"1", true},
		{"0", false},
		{"yes", true},
		{"no", false},
		{"", false},
		{"null", false},
		{"anything", true},
		{"production", true},
		{"{{unresolved}}", false},
	}

	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			result := isTruthy(tt.value)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRunner_SSE_EventStream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("event: message\ndata: hello\n\nevent: update\ndata: world\n\n"))
	}))
	defer server.Close()

	content := `### SSE Test
# @name sseTest

GET ` + server.URL + `/events
Accept: text/event-stream

>>>
expect status 200
<<<`

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.http")
	err := os.WriteFile(testFile, []byte(content), 0644)
	require.NoError(t, err)

	r := NewRunner(&Config{})
	result, err := r.RunFile(testFile)

	require.NoError(t, err)
	assert.Equal(t, 1, result.Passed)
	assert.Equal(t, 0, result.Failed)
	require.Len(t, result.Results, 1)
	assert.True(t, result.Results[0].Passed)

	// Verify SSE events were parsed
	require.Len(t, result.Results[0].SSEEvents, 2)
	assert.Equal(t, "message", result.Results[0].SSEEvents[0].Type)
	assert.Equal(t, "hello", result.Results[0].SSEEvents[0].Data)
	assert.Equal(t, "update", result.Results[0].SSEEvents[1].Type)
	assert.Equal(t, "world", result.Results[0].SSEEvents[1].Data)
}

func TestRunner_SSE_NonEventStream(t *testing.T) {
	// Verify that non-SSE responses don't get SSE parsing
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status": "ok"}`))
	}))
	defer server.Close()

	content := `### JSON Test
GET ` + server.URL + `/api

>>>
expect status 200
<<<`

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.http")
	err := os.WriteFile(testFile, []byte(content), 0644)
	require.NoError(t, err)

	r := NewRunner(&Config{})
	result, err := r.RunFile(testFile)

	require.NoError(t, err)
	assert.Equal(t, 1, result.Passed)
	require.Len(t, result.Results, 1)
	assert.Nil(t, result.Results[0].SSEEvents)
}

func TestRunner_SSE_MultiLineData(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("id: 42\nevent: payload\ndata: line1\ndata: line2\n\n"))
	}))
	defer server.Close()

	content := `### SSE Multi-line
GET ` + server.URL + `/events
Accept: text/event-stream

>>>
expect status 200
<<<`

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.http")
	err := os.WriteFile(testFile, []byte(content), 0644)
	require.NoError(t, err)

	r := NewRunner(&Config{})
	result, err := r.RunFile(testFile)

	require.NoError(t, err)
	require.Len(t, result.Results, 1)
	require.Len(t, result.Results[0].SSEEvents, 1)

	ev := result.Results[0].SSEEvents[0]
	assert.Equal(t, "42", ev.ID)
	assert.Equal(t, "payload", ev.Type)
	assert.Equal(t, "line1\nline2", ev.Data)
}
