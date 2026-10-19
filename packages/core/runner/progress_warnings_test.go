package runner

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A request's warnings travel with its "completed" progress event (both the
// sequential and the parallel path), not only inside RequestResult.
func TestProgressEvent_CarriesWarnings(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	file := filepath.Join(t.TempDir(), "warn.http")
	src := "### warn\nGET " + server.URL + "/a\nX-Token: {{nobody.token}}\n"
	require.NoError(t, os.WriteFile(file, []byte(src), 0o644))

	for _, parallel := range []bool{false, true} {
		var mu sync.Mutex
		var events []ProgressEvent
		r := NewRunner(&Config{
			Parallel: parallel,
			OnProgress: func(e ProgressEvent) {
				mu.Lock()
				events = append(events, e)
				mu.Unlock()
			},
		})
		result, err := r.RunFile(file)
		require.NoError(t, err)
		require.Len(t, result.Results, 1)
		require.NotEmpty(t, result.Results[0].Warnings, "parallel=%v", parallel)

		mu.Lock()
		require.Len(t, events, 2, "parallel=%v", parallel)
		assert.Empty(t, events[0].Warnings, "started event carries no warnings")
		assert.Equal(t, "completed", events[1].Status)
		assert.Equal(t, result.Results[0].Warnings, events[1].Warnings, "parallel=%v", parallel)
		mu.Unlock()
	}
}
