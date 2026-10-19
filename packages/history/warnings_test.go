package history

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

// Warnings are stored with their result, listed back in order, and deleted
// with the run.
func TestRecordCompleteRun_Warnings(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	id, err := s.RecordCompleteRun(ctx, RunRecord{
		FilePath: "a.http",
		Results: []ResultRecord{
			{Name: "warn", Method: "GET", URL: "http://x", Passed: true, Warnings: []string{"first", "second"}},
			{Name: "clean", Method: "GET", URL: "http://y", Passed: true},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	results, err := s.Queries().ListResultsByRun(ctx, id)
	if err != nil || len(results) != 2 {
		t.Fatalf("results = %v, %v", results, err)
	}
	got, err := s.Queries().ListWarningsByResult(ctx, results[0].ID)
	if err != nil || len(got) != 2 || got[0].Message != "first" || got[1].Message != "second" {
		t.Fatalf("warnings = %+v, %v", got, err)
	}
	if none, _ := s.Queries().ListWarningsByResult(ctx, results[1].ID); len(none) != 0 {
		t.Fatalf("a result without warnings has %+v", none)
	}

	if err := s.Queries().DeleteRun(ctx, id); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := s.DB().QueryRowContext(ctx, "SELECT COUNT(*) FROM result_warnings").Scan(&n); err != nil || n != 0 {
		t.Fatalf("warnings left after deleting the run: %d, %v", n, err)
	}
}

// A database created before warnings were stored keeps its runs and picks up
// the new table when it is opened: no ALTER, no data loss.
func TestNewStore_UpgradesDatabaseWithoutWarningsTable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.db")
	old, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{
		`CREATE TABLE runs (id INTEGER PRIMARY KEY, file_path TEXT NOT NULL, environment TEXT, started_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, finished_at DATETIME, duration_ms INTEGER NOT NULL DEFAULT 0, passed INTEGER NOT NULL DEFAULT 0, failed INTEGER NOT NULL DEFAULT 0, skipped INTEGER NOT NULL DEFAULT 0, total INTEGER NOT NULL DEFAULT 0)`,
		`CREATE TABLE results (id INTEGER PRIMARY KEY, run_id INTEGER NOT NULL REFERENCES runs(id) ON DELETE CASCADE, request_name TEXT NOT NULL, method TEXT NOT NULL, url TEXT NOT NULL, status_code INTEGER, duration_ms INTEGER NOT NULL DEFAULT 0, passed BOOLEAN NOT NULL DEFAULT false, skipped BOOLEAN NOT NULL DEFAULT false, error TEXT, description TEXT, body_preview TEXT, created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE assertions (id INTEGER PRIMARY KEY, result_id INTEGER NOT NULL REFERENCES results(id) ON DELETE CASCADE, operator TEXT NOT NULL, subject TEXT NOT NULL, expected TEXT, actual TEXT, passed BOOLEAN NOT NULL DEFAULT false, message TEXT)`,
		`INSERT INTO runs (file_path, passed, total) VALUES ('old.http', 1, 1)`,
		`INSERT INTO results (run_id, request_name, method, url, passed) VALUES (1, 'old', 'GET', 'http://old', true)`,
	} {
		if _, err := old.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	_ = old.Close()

	s, err := NewStore(path)
	if err != nil {
		t.Fatalf("opening an old database: %v", err)
	}
	defer s.Close()
	ctx := context.Background()
	results, err := s.Queries().ListResultsByRun(ctx, 1)
	if err != nil || len(results) != 1 || results[0].RequestName != "old" {
		t.Fatalf("old rows lost: %v, %v", results, err)
	}
	if w, err := s.Queries().ListWarningsByResult(ctx, results[0].ID); err != nil || len(w) != 0 {
		t.Fatalf("old result warnings = %v, %v; want none", w, err)
	}
	id, err := s.RecordCompleteRun(ctx, RunRecord{FilePath: "new.http", Results: []ResultRecord{{Name: "n", Method: "GET", URL: "http://n", Warnings: []string{"w"}}}})
	if err != nil {
		t.Fatal(err)
	}
	newResults, _ := s.Queries().ListResultsByRun(ctx, id)
	if w, _ := s.Queries().ListWarningsByResult(ctx, newResults[0].ID); len(w) != 1 {
		t.Fatalf("new run warnings = %v", w)
	}
}
