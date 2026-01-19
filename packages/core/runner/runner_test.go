package runner

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

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
		result := hasAnyTag(tt.tags, tt.filters)
		assert.Equal(t, tt.expected, result)
	}
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

		r := NewRunner(&Config{})
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

		r := NewRunner(&Config{})
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

		r := NewRunner(&Config{})
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

		r := NewRunner(&Config{})
		result, err := r.RunFile(testFile)

		require.NoError(t, err)
		assert.True(t, result.Results[0].Passed)

		// Verify hooks executed in order
		data, err := os.ReadFile(orderFile)
		require.NoError(t, err)
		assert.Equal(t, "1\n2\n", string(data))
	})
}
