package history

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// newTestStore creates a Store backed by a fresh in-memory SQLite database.
// It registers a cleanup function that closes the store when the test finishes.
func newTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	s, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore(%q): %v", dbPath, err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// ---------------------------------------------------------------------------
// 1. NewStore
// ---------------------------------------------------------------------------

func TestNewStore_CreatesDatabase(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "history.db")

	s, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer s.Close()

	// File should exist on disk.
	if _, err := os.Stat(dbPath); err != nil {
		t.Fatalf("database file not created: %v", err)
	}

	// DB and Queries accessors should return non-nil values.
	if s.DB() == nil {
		t.Fatal("DB() returned nil")
	}
	if s.Queries() == nil {
		t.Fatal("Queries() returned nil")
	}
}

func TestNewStore_SchemaApplied(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	// Verify the three tables exist by querying sqlite_master.
	rows, err := s.DB().QueryContext(ctx,
		`SELECT name FROM sqlite_master WHERE type='table' AND name IN ('runs','results','assertions') ORDER BY name`)
	if err != nil {
		t.Fatalf("query sqlite_master: %v", err)
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan: %v", err)
		}
		tables = append(tables, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows.Err: %v", err)
	}

	expected := []string{"assertions", "results", "runs"}
	if len(tables) != len(expected) {
		t.Fatalf("expected tables %v, got %v", expected, tables)
	}
	for i := range expected {
		if tables[i] != expected[i] {
			t.Errorf("table[%d]: expected %q, got %q", i, expected[i], tables[i])
		}
	}
}

func TestNewStore_InvalidPath(t *testing.T) {
	// A path inside a non-existent directory should fail.
	_, err := NewStore("/no/such/directory/test.db")
	if err == nil {
		t.Fatal("expected error for invalid DB path, got nil")
	}
}

func TestNewStore_IdempotentSchema(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "history.db")

	// Open twice on the same path -- schema uses IF NOT EXISTS, so the second
	// call should succeed without error.
	s1, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("first NewStore: %v", err)
	}
	s1.Close()

	s2, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("second NewStore: %v", err)
	}
	s2.Close()
}

// ---------------------------------------------------------------------------
// 2. RecordRun
// ---------------------------------------------------------------------------

func TestRecordRun(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	id, err := s.RecordRun(ctx, "api.http", "staging")
	if err != nil {
		t.Fatalf("RecordRun: %v", err)
	}
	if id <= 0 {
		t.Fatalf("expected positive ID, got %d", id)
	}

	run, err := s.q.GetRun(ctx, id)
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if run.FilePath != "api.http" {
		t.Errorf("FilePath: expected %q, got %q", "api.http", run.FilePath)
	}
	if !run.Environment.Valid || run.Environment.String != "staging" {
		t.Errorf("Environment: expected staging, got %v", run.Environment)
	}
}

func TestRecordRun_EmptyEnvironment(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	id, err := s.RecordRun(ctx, "test.http", "")
	if err != nil {
		t.Fatalf("RecordRun: %v", err)
	}

	run, err := s.q.GetRun(ctx, id)
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if run.Environment.Valid {
		t.Errorf("expected NULL environment, got %q", run.Environment.String)
	}
}

func TestRecordRun_MultipleRuns(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	ids := make(map[int64]bool)
	for i := 0; i < 5; i++ {
		id, err := s.RecordRun(ctx, "test.http", "")
		if err != nil {
			t.Fatalf("RecordRun[%d]: %v", i, err)
		}
		if ids[id] {
			t.Fatalf("duplicate ID %d on iteration %d", id, i)
		}
		ids[id] = true
	}
}

// ---------------------------------------------------------------------------
// 3. FinishRun
// ---------------------------------------------------------------------------

func TestFinishRun(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	id, err := s.RecordRun(ctx, "api.http", "production")
	if err != nil {
		t.Fatalf("RecordRun: %v", err)
	}

	dur := 1500 * time.Millisecond
	if err := s.FinishRun(ctx, id, dur, 8, 1, 2, 11); err != nil {
		t.Fatalf("FinishRun: %v", err)
	}

	run, err := s.q.GetRun(ctx, id)
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}

	if !run.FinishedAt.Valid {
		t.Fatal("FinishedAt should be set after FinishRun")
	}
	if run.DurationMs != 1500 {
		t.Errorf("DurationMs: expected 1500, got %d", run.DurationMs)
	}
	if run.Passed != 8 {
		t.Errorf("Passed: expected 8, got %d", run.Passed)
	}
	if run.Failed != 1 {
		t.Errorf("Failed: expected 1, got %d", run.Failed)
	}
	if run.Skipped != 2 {
		t.Errorf("Skipped: expected 2, got %d", run.Skipped)
	}
	if run.Total != 11 {
		t.Errorf("Total: expected 11, got %d", run.Total)
	}
}

func TestFinishRun_NonExistentID(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	// Finishing a run that doesn't exist is a no-op (UPDATE WHERE id=? matches 0 rows).
	// It should not return an error.
	err := s.FinishRun(ctx, 99999, time.Second, 0, 0, 0, 0)
	if err != nil {
		t.Fatalf("FinishRun on non-existent ID: %v", err)
	}
}

// ---------------------------------------------------------------------------
// 4. RecordResult
// ---------------------------------------------------------------------------

func TestRecordResult(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	runID, err := s.RecordRun(ctx, "api.http", "")
	if err != nil {
		t.Fatalf("RecordRun: %v", err)
	}

	resID, err := s.RecordResult(ctx, runID,
		"Get Users", "GET", "https://api.example.com/users",
		200, 123, true, false,
		"", "List all users", `{"users":[]}`,
	)
	if err != nil {
		t.Fatalf("RecordResult: %v", err)
	}
	if resID <= 0 {
		t.Fatalf("expected positive result ID, got %d", resID)
	}

	results, err := s.q.ListResultsByRun(ctx, runID)
	if err != nil {
		t.Fatalf("ListResultsByRun: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	r := results[0]
	if r.RequestName != "Get Users" {
		t.Errorf("RequestName: expected %q, got %q", "Get Users", r.RequestName)
	}
	if r.Method != "GET" {
		t.Errorf("Method: expected GET, got %q", r.Method)
	}
	if r.Url != "https://api.example.com/users" {
		t.Errorf("URL: expected %q, got %q", "https://api.example.com/users", r.Url)
	}
	if !r.StatusCode.Valid || r.StatusCode.Int64 != 200 {
		t.Errorf("StatusCode: expected 200, got %v", r.StatusCode)
	}
	if r.DurationMs != 123 {
		t.Errorf("DurationMs: expected 123, got %d", r.DurationMs)
	}
	if !r.Passed {
		t.Error("Passed: expected true")
	}
	if r.Skipped {
		t.Error("Skipped: expected false")
	}
	if r.Error.Valid {
		t.Errorf("Error: expected NULL, got %q", r.Error.String)
	}
	if !r.Description.Valid || r.Description.String != "List all users" {
		t.Errorf("Description: expected %q, got %v", "List all users", r.Description)
	}
	if !r.BodyPreview.Valid || r.BodyPreview.String != `{"users":[]}` {
		t.Errorf("BodyPreview: expected %q, got %v", `{"users":[]}`, r.BodyPreview)
	}
}

func TestRecordResult_WithError(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	runID, _ := s.RecordRun(ctx, "api.http", "")
	resID, err := s.RecordResult(ctx, runID,
		"Broken", "POST", "https://api.example.com/broken",
		0, 50, false, false,
		"connection refused", "", "",
	)
	if err != nil {
		t.Fatalf("RecordResult: %v", err)
	}

	results, _ := s.q.ListResultsByRun(ctx, runID)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if r.ID != resID {
		t.Errorf("ID mismatch: expected %d, got %d", resID, r.ID)
	}
	if !r.Error.Valid || r.Error.String != "connection refused" {
		t.Errorf("Error: expected %q, got %v", "connection refused", r.Error)
	}
	// status_code 0 becomes NULL via nullInt64.
	if r.StatusCode.Valid {
		t.Errorf("StatusCode: expected NULL for 0, got %d", r.StatusCode.Int64)
	}
}

func TestRecordResult_Skipped(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	runID, _ := s.RecordRun(ctx, "test.http", "")
	_, err := s.RecordResult(ctx, runID,
		"Conditional", "GET", "https://api.example.com/skip",
		0, 0, false, true,
		"", "skipped by condition", "",
	)
	if err != nil {
		t.Fatalf("RecordResult: %v", err)
	}

	results, _ := s.q.ListResultsByRun(ctx, runID)
	if !results[0].Skipped {
		t.Error("expected Skipped=true")
	}
}

// ---------------------------------------------------------------------------
// 5. RecordAssertions
// ---------------------------------------------------------------------------

func TestRecordAssertions(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	runID, _ := s.RecordRun(ctx, "api.http", "")
	resID, _ := s.RecordResult(ctx, runID,
		"Test", "GET", "https://example.com", 200, 10, true, false, "", "", "",
	)

	assertions := []AssertionRecord{
		{Operator: "equals", Subject: "status", Expected: "200", Actual: "200", Passed: true, Message: ""},
		{Operator: "contains", Subject: "body", Expected: "ok", Actual: `{"status":"ok"}`, Passed: true, Message: "body contains ok"},
		{Operator: "equals", Subject: "header.Content-Type", Expected: "application/json", Actual: "text/plain", Passed: false, Message: "content type mismatch"},
	}

	if err := s.RecordAssertions(ctx, resID, assertions); err != nil {
		t.Fatalf("RecordAssertions: %v", err)
	}

	rows, err := s.q.ListAssertionsByResult(ctx, resID)
	if err != nil {
		t.Fatalf("ListAssertionsByResult: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("expected 3 assertions, got %d", len(rows))
	}

	// Verify first assertion.
	a := rows[0]
	if a.Operator != "equals" {
		t.Errorf("assertion[0].Operator: expected %q, got %q", "equals", a.Operator)
	}
	if a.Subject != "status" {
		t.Errorf("assertion[0].Subject: expected %q, got %q", "status", a.Subject)
	}
	if !a.Passed {
		t.Error("assertion[0] should be passed")
	}

	// Verify third (failed) assertion.
	a2 := rows[2]
	if a2.Passed {
		t.Error("assertion[2] should not be passed")
	}
	if !a2.Message.Valid || a2.Message.String != "content type mismatch" {
		t.Errorf("assertion[2].Message: expected %q, got %v", "content type mismatch", a2.Message)
	}
}

func TestRecordAssertions_EmptySlice(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	runID, _ := s.RecordRun(ctx, "api.http", "")
	resID, _ := s.RecordResult(ctx, runID,
		"NoAssert", "GET", "https://example.com", 200, 5, true, false, "", "", "",
	)

	// Empty assertions slice should commit an empty transaction without error.
	if err := s.RecordAssertions(ctx, resID, nil); err != nil {
		t.Fatalf("RecordAssertions(nil): %v", err)
	}

	rows, _ := s.q.ListAssertionsByResult(ctx, resID)
	if len(rows) != 0 {
		t.Errorf("expected 0 assertions, got %d", len(rows))
	}
}

// ---------------------------------------------------------------------------
// 6. Full flow
// ---------------------------------------------------------------------------

func TestFullFlow(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	// 1. Record a run.
	runID, err := s.RecordRun(ctx, "integration.http", "ci")
	if err != nil {
		t.Fatalf("RecordRun: %v", err)
	}

	// 2. Record two results.
	res1ID, err := s.RecordResult(ctx, runID,
		"GET /health", "GET", "https://api.example.com/health",
		200, 45, true, false,
		"", "health check", `{"status":"ok"}`,
	)
	if err != nil {
		t.Fatalf("RecordResult(1): %v", err)
	}

	res2ID, err := s.RecordResult(ctx, runID,
		"POST /data", "POST", "https://api.example.com/data",
		500, 320, false, false,
		"internal server error", "create data", `{"error":"boom"}`,
	)
	if err != nil {
		t.Fatalf("RecordResult(2): %v", err)
	}

	// 3. Record assertions for each result.
	if err := s.RecordAssertions(ctx, res1ID, []AssertionRecord{
		{Operator: "equals", Subject: "status", Expected: "200", Actual: "200", Passed: true},
	}); err != nil {
		t.Fatalf("RecordAssertions(1): %v", err)
	}

	if err := s.RecordAssertions(ctx, res2ID, []AssertionRecord{
		{Operator: "equals", Subject: "status", Expected: "201", Actual: "500", Passed: false, Message: "expected 201"},
		{Operator: "exists", Subject: "body.id", Expected: "", Actual: "", Passed: false, Message: "body.id missing"},
	}); err != nil {
		t.Fatalf("RecordAssertions(2): %v", err)
	}

	// 4. Finish the run.
	dur := 365 * time.Millisecond
	if err := s.FinishRun(ctx, runID, dur, 1, 1, 0, 2); err != nil {
		t.Fatalf("FinishRun: %v", err)
	}

	// 5. Verify via GetRun.
	run, err := s.q.GetRun(ctx, runID)
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if run.FilePath != "integration.http" {
		t.Errorf("FilePath: %q", run.FilePath)
	}
	if !run.Environment.Valid || run.Environment.String != "ci" {
		t.Errorf("Environment: %v", run.Environment)
	}
	if run.Passed != 1 || run.Failed != 1 || run.Total != 2 {
		t.Errorf("stats: passed=%d failed=%d total=%d", run.Passed, run.Failed, run.Total)
	}
	if run.DurationMs != 365 {
		t.Errorf("DurationMs: expected 365, got %d", run.DurationMs)
	}
	if !run.FinishedAt.Valid {
		t.Error("FinishedAt should be set")
	}

	// 6. Verify results via ListResultsByRun.
	results, err := s.q.ListResultsByRun(ctx, runID)
	if err != nil {
		t.Fatalf("ListResultsByRun: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].RequestName != "GET /health" {
		t.Errorf("result[0].RequestName: %q", results[0].RequestName)
	}
	if results[1].RequestName != "POST /data" {
		t.Errorf("result[1].RequestName: %q", results[1].RequestName)
	}

	// 7. Verify assertions via ListAssertionsByResult.
	a1, _ := s.q.ListAssertionsByResult(ctx, res1ID)
	if len(a1) != 1 {
		t.Fatalf("expected 1 assertion for res1, got %d", len(a1))
	}
	a2, _ := s.q.ListAssertionsByResult(ctx, res2ID)
	if len(a2) != 2 {
		t.Fatalf("expected 2 assertions for res2, got %d", len(a2))
	}
}

// ---------------------------------------------------------------------------
// 7. CountRuns, DeleteRun, ClearAllRuns, DeleteRunsBefore
// ---------------------------------------------------------------------------

func TestCountRuns(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	count, err := s.q.CountRuns(ctx)
	if err != nil {
		t.Fatalf("CountRuns: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0, got %d", count)
	}

	for i := 0; i < 3; i++ {
		if _, err := s.RecordRun(ctx, "test.http", ""); err != nil {
			t.Fatalf("RecordRun[%d]: %v", i, err)
		}
	}

	count, err = s.q.CountRuns(ctx)
	if err != nil {
		t.Fatalf("CountRuns: %v", err)
	}
	if count != 3 {
		t.Errorf("expected 3, got %d", count)
	}
}

func TestDeleteRun(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	id1, _ := s.RecordRun(ctx, "a.http", "")
	id2, _ := s.RecordRun(ctx, "b.http", "")

	if err := s.q.DeleteRun(ctx, id1); err != nil {
		t.Fatalf("DeleteRun: %v", err)
	}

	count, _ := s.q.CountRuns(ctx)
	if count != 1 {
		t.Errorf("expected 1 run remaining, got %d", count)
	}

	// The remaining run should be id2.
	run, err := s.q.GetRun(ctx, id2)
	if err != nil {
		t.Fatalf("GetRun(id2): %v", err)
	}
	if run.FilePath != "b.http" {
		t.Errorf("expected b.http, got %q", run.FilePath)
	}

	// Deleted run should not be found.
	_, err = s.q.GetRun(ctx, id1)
	if err != sql.ErrNoRows {
		t.Errorf("expected sql.ErrNoRows for deleted run, got %v", err)
	}
}

func TestDeleteRun_CascadesResults(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	runID, _ := s.RecordRun(ctx, "cascade.http", "")
	resID, _ := s.RecordResult(ctx, runID,
		"Test", "GET", "https://example.com", 200, 10, true, false, "", "", "",
	)
	_ = s.RecordAssertions(ctx, resID, []AssertionRecord{
		{Operator: "equals", Subject: "status", Expected: "200", Actual: "200", Passed: true},
	})

	// Delete the run -- should cascade to results and assertions.
	if err := s.q.DeleteRun(ctx, runID); err != nil {
		t.Fatalf("DeleteRun: %v", err)
	}

	results, _ := s.q.ListResultsByRun(ctx, runID)
	if len(results) != 0 {
		t.Errorf("expected 0 results after cascade delete, got %d", len(results))
	}

	assertions, _ := s.q.ListAssertionsByResult(ctx, resID)
	if len(assertions) != 0 {
		t.Errorf("expected 0 assertions after cascade delete, got %d", len(assertions))
	}
}

func TestClearAllRuns(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		if _, err := s.RecordRun(ctx, "test.http", ""); err != nil {
			t.Fatalf("RecordRun: %v", err)
		}
	}

	if err := s.q.ClearAllRuns(ctx); err != nil {
		t.Fatalf("ClearAllRuns: %v", err)
	}

	count, _ := s.q.CountRuns(ctx)
	if count != 0 {
		t.Errorf("expected 0 after ClearAllRuns, got %d", count)
	}
}

func TestDeleteRunsBefore(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	// Insert runs with explicit timestamps so we control the values.
	// CURRENT_TIMESTAMP in SQLite produces an ISO8601 text value, and the
	// modernc.org/sqlite driver serializes time.Time differently. To make
	// DeleteRunsBefore work reliably, we insert with known text timestamps
	// and verify the deletion logic.
	_, err := s.DB().ExecContext(ctx,
		`INSERT INTO runs (file_path, started_at) VALUES ('old.http', '2020-01-01 00:00:00')`)
	if err != nil {
		t.Fatalf("insert old run: %v", err)
	}
	_, err = s.DB().ExecContext(ctx,
		`INSERT INTO runs (file_path, started_at) VALUES ('mid.http', '2023-06-15 12:00:00')`)
	if err != nil {
		t.Fatalf("insert mid run: %v", err)
	}
	_, err = s.DB().ExecContext(ctx,
		`INSERT INTO runs (file_path, started_at) VALUES ('new.http', '2025-12-31 23:59:59')`)
	if err != nil {
		t.Fatalf("insert new run: %v", err)
	}

	count, _ := s.q.CountRuns(ctx)
	if count != 3 {
		t.Fatalf("expected 3 runs, got %d", count)
	}

	// Delete runs before 2023-01-01 -- only the 2020 run should be removed.
	cutoff := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := s.q.DeleteRunsBefore(ctx, cutoff); err != nil {
		t.Fatalf("DeleteRunsBefore(2023): %v", err)
	}
	count, _ = s.q.CountRuns(ctx)
	if count != 2 {
		t.Errorf("expected 2 runs after deleting pre-2023, got %d", count)
	}

	// Delete runs before 2030 -- all remaining should be removed.
	cutoff2 := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := s.q.DeleteRunsBefore(ctx, cutoff2); err != nil {
		t.Fatalf("DeleteRunsBefore(2030): %v", err)
	}
	count, _ = s.q.CountRuns(ctx)
	if count != 0 {
		t.Errorf("expected 0 runs after deleting pre-2030, got %d", count)
	}
}

// ---------------------------------------------------------------------------
// ListRuns
// ---------------------------------------------------------------------------

func TestListRuns(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	// Insert 5 runs.
	for i := 0; i < 5; i++ {
		if _, err := s.RecordRun(ctx, "test.http", ""); err != nil {
			t.Fatalf("RecordRun: %v", err)
		}
	}

	// List with limit 3, offset 0.
	runs, err := s.q.ListRuns(ctx, ListRunsParams{Limit: 3, Offset: 0})
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	if len(runs) != 3 {
		t.Errorf("expected 3 runs, got %d", len(runs))
	}

	// List with limit 10, offset 3 -- should get remaining 2.
	runs, err = s.q.ListRuns(ctx, ListRunsParams{Limit: 10, Offset: 3})
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	if len(runs) != 2 {
		t.Errorf("expected 2 runs, got %d", len(runs))
	}
}

// ---------------------------------------------------------------------------
// 8. Error cases
// ---------------------------------------------------------------------------

func TestRecordRun_CancelledContext(t *testing.T) {
	s := newTestStore(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := s.RecordRun(ctx, "api.http", "")
	if err == nil {
		t.Fatal("expected error with cancelled context, got nil")
	}
}

func TestRecordResult_CancelledContext(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	runID, err := s.RecordRun(ctx, "api.http", "")
	if err != nil {
		t.Fatalf("RecordRun: %v", err)
	}

	cancelledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = s.RecordResult(cancelledCtx, runID,
		"Test", "GET", "https://example.com", 200, 10, true, false, "", "", "",
	)
	if err == nil {
		t.Fatal("expected error with cancelled context, got nil")
	}
}

func TestRecordAssertions_CancelledContext(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	runID, _ := s.RecordRun(ctx, "api.http", "")
	resID, _ := s.RecordResult(ctx, runID,
		"Test", "GET", "https://example.com", 200, 10, true, false, "", "", "",
	)

	cancelledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	err := s.RecordAssertions(cancelledCtx, resID, []AssertionRecord{
		{Operator: "equals", Subject: "status", Expected: "200", Actual: "200", Passed: true},
	})
	if err == nil {
		t.Fatal("expected error with cancelled context, got nil")
	}
}

func TestFinishRun_CancelledContext(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	runID, _ := s.RecordRun(ctx, "api.http", "")

	cancelledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	err := s.FinishRun(cancelledCtx, runID, time.Second, 1, 0, 0, 1)
	if err == nil {
		t.Fatal("expected error with cancelled context, got nil")
	}
}

func TestCountRuns_CancelledContext(t *testing.T) {
	s := newTestStore(t)

	cancelledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := s.q.CountRuns(cancelledCtx)
	if err == nil {
		t.Fatal("expected error with cancelled context, got nil")
	}
}

// ---------------------------------------------------------------------------
// Close
// ---------------------------------------------------------------------------

func TestClose(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	s, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// After closing, operations should fail.
	_, err = s.RecordRun(context.Background(), "test.http", "")
	if err == nil {
		t.Fatal("expected error after Close, got nil")
	}
}

// ---------------------------------------------------------------------------
// Helpers: nullString and nullInt64
// ---------------------------------------------------------------------------

func TestNullString(t *testing.T) {
	ns := nullString("")
	if ns.Valid {
		t.Error("nullString(\"\") should be invalid")
	}

	ns = nullString("hello")
	if !ns.Valid {
		t.Error("nullString(\"hello\") should be valid")
	}
	if ns.String != "hello" {
		t.Errorf("expected %q, got %q", "hello", ns.String)
	}
}

func TestNullInt64(t *testing.T) {
	ni := nullInt64(0)
	if ni.Valid {
		t.Error("nullInt64(0) should be invalid")
	}

	ni = nullInt64(42)
	if !ni.Valid {
		t.Error("nullInt64(42) should be valid")
	}
	if ni.Int64 != 42 {
		t.Errorf("expected 42, got %d", ni.Int64)
	}
}
