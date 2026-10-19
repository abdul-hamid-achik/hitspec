package runner

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/abdul-hamid-achik/hitspec/v2/packages/history"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The CLI runner stores each request's warnings with its history result.
func TestRunner_HistoryKeepsWarnings(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer server.Close()

	dir := t.TempDir()
	store, err := history.NewStore(filepath.Join(dir, "history.db"))
	require.NoError(t, err)
	defer store.Close()
	file := filepath.Join(dir, "w.http")
	require.NoError(t, os.WriteFile(file, []byte("### warn\n# @name warn\nGET "+server.URL+"/w\nX-Token: {{nobody.token}}\n"), 0o644))

	r := NewRunner(&Config{HistoryStore: store})
	res, err := r.RunFile(file)
	require.NoError(t, err)
	require.NotEmpty(t, res.Results[0].Warnings)
	r.WaitHistory()

	ctx := context.Background()
	runs, err := store.Queries().ListRuns(ctx, history.ListRunsParams{Limit: 10})
	require.NoError(t, err)
	require.Len(t, runs, 1)
	results, err := store.Queries().ListResultsByRun(ctx, runs[0].ID)
	require.NoError(t, err)
	require.Len(t, results, 1)
	warnings, err := store.Queries().ListWarningsByResult(ctx, results[0].ID)
	require.NoError(t, err)
	require.Len(t, warnings, len(res.Results[0].Warnings))
	assert.Equal(t, res.Results[0].Warnings[0], warnings[0].Message)
}
