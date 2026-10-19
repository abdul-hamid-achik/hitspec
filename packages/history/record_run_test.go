package history

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestRecordCompleteRun(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	id, err := s.RecordCompleteRun(ctx, RunRecord{
		FilePath:    "api/users.http",
		Environment: "dev",
		Duration:    250 * time.Millisecond,
		Results: []ResultRecord{
			{Name: "ok", Method: "GET", URL: "http://x/ok", StatusCode: 200, DurationMs: 12, Passed: true, BodyPreview: "{}",
				Assertions: []AssertionRecord{{Operator: "==", Subject: "status", Expected: "200", Actual: "200", Passed: true}}},
			{Name: "bad", Method: "GET", URL: "http://x/bad", StatusCode: 500, DurationMs: 5, Error: "boom",
				Assertions: []AssertionRecord{{Operator: "==", Subject: "status", Expected: "200", Actual: "500"}}},
			{Name: "skipped", Method: "GET", URL: "http://x/skip", Skipped: true},
		},
	})
	if err != nil {
		t.Fatalf("RecordCompleteRun: %v", err)
	}

	run, err := s.Queries().GetRun(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if run.Passed != 1 || run.Failed != 1 || run.Skipped != 1 || run.Total != 3 || run.DurationMs != 250 || !run.FinishedAt.Valid {
		t.Fatalf("run summary = %+v", run)
	}
	results, err := s.Queries().ListResultsByRun(ctx, id)
	if err != nil || len(results) != 3 {
		t.Fatalf("results = %d, err %v", len(results), err)
	}
	asserts, err := s.Queries().ListAssertionsByResult(ctx, results[0].ID)
	if err != nil || len(asserts) != 1 || !asserts[0].Passed {
		t.Fatalf("assertions = %+v, err %v", asserts, err)
	}
}

// A failure part-way through must leave nothing behind: no run row without its
// results.
func TestRecordCompleteRunIsAtomic(t *testing.T) {
	s := newTestStore(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.RecordCompleteRun(ctx, RunRecord{FilePath: "a.http", Results: []ResultRecord{{Name: "x"}}}); err == nil {
		t.Fatal("expected an error from a cancelled context")
	}
	n, err := s.Queries().CountRuns(context.Background())
	if err != nil || n != 0 {
		t.Fatalf("runs after failed record = %d, err %v; want 0", n, err)
	}
}

func TestNewStoreTightensPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permissions")
	}
	dir := filepath.Join(t.TempDir(), ".hitspec")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	db := filepath.Join(dir, "history.db")
	if err := os.WriteFile(db, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := NewStore(db)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.RecordCompleteRun(context.Background(), RunRecord{FilePath: "a.http"}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })

	for _, p := range []string{db, db + "-wal", db + "-shm"} {
		fi, err := os.Stat(p)
		if err != nil {
			continue // sqlite may not have created every sidecar
		}
		if fi.Mode().Perm()&0o077 != 0 {
			t.Errorf("%s mode %v is accessible to group/other", filepath.Base(p), fi.Mode().Perm())
		}
	}
	fi, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o700 {
		t.Errorf("default ~/.hitspec-style directory mode = %v, want 0700", fi.Mode().Perm())
	}
}

// A user-chosen --history-db path must never change the permissions of an
// arbitrary parent directory (it may be the workspace or the cwd).
func TestNewStoreLeavesCustomDirAlone(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permissions")
	}
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	s, err := NewStore(filepath.Join(dir, "h.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	fi, _ := os.Stat(dir)
	if fi.Mode().Perm() != 0o755 {
		t.Errorf("custom dir mode changed to %v", fi.Mode().Perm())
	}
}
