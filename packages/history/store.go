package history

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"time"

	// SQLite driver (pure Go, no CGo required)
	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schemaSQL string

// Store provides high-level operations for run history persistence.
type Store struct {
	db *sql.DB
	q  *Queries
}

// DefaultPath returns the default database location, ~/.hitspec/history.db.
// NewStore creates the directory (0700) when it is missing.
func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".hitspec", "history.db"), nil
}

// NewStore opens (or creates) a SQLite database at dbPath and applies the schema.
// The database may hold request URLs/bodies (potentially sensitive), so the
// file (and its -wal/-shm sidecars) is kept at 0600, including a database that
// an older version created world-readable, and the default ~/.hitspec directory
// at 0700. The parent of a custom path is created 0700 when missing but never
// re-permissioned: it may be the workspace or the current directory.
func NewStore(dbPath string) (*Store, error) {
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("history: create directory: %w", err)
	}
	if filepath.Base(dir) == ".hitspec" {
		_ = os.Chmod(dir, 0o700)
	}
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		if f, err := os.OpenFile(dbPath, os.O_CREATE|os.O_RDWR, 0o600); err == nil {
			_ = f.Close()
		} else {
			return nil, fmt.Errorf("history: create database: %w", err)
		}
	}
	restrictDBFiles(dbPath)
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("history: open database: %w", err)
	}
	// SQLite allows only one writer at a time. Serializing to a single connection
	// avoids SQLITE_BUSY from the app's own concurrent access (the run recorder
	// writes asynchronously while the UI reads/deletes) and ensures the
	// connection-level pragmas below apply to every query.
	db.SetMaxOpenConns(1)

	// Enable WAL mode and foreign keys for better concurrency and referential
	// integrity. busy_timeout makes a writer wait for the lock (history is
	// written asynchronously after a run, so it can overlap with reads/deletes)
	// instead of failing immediately with SQLITE_BUSY.
	for _, pragma := range []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA busy_timeout=5000",
		"PRAGMA foreign_keys=ON",
	} {
		if _, err := db.Exec(pragma); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("history: %s: %w", pragma, err)
		}
	}

	if _, err := db.Exec(schemaSQL); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("history: apply schema: %w", err)
	}

	restrictDBFiles(dbPath) // the WAL sidecars exist now
	return &Store{db: db, q: New(db)}, nil
}

// restrictDBFiles chmods the database and its SQLite sidecars to 0600. Missing
// files and chmod failures (e.g. a filesystem without POSIX modes) are ignored.
func restrictDBFiles(dbPath string) {
	for _, p := range []string{dbPath, dbPath + "-wal", dbPath + "-shm"} {
		if fi, err := os.Stat(p); err == nil && fi.Mode().Perm()&0o077 != 0 {
			_ = os.Chmod(p, 0o600)
		}
	}
}

// Close closes the underlying database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// DB returns the underlying *sql.DB for advanced use cases.
func (s *Store) DB() *sql.DB {
	return s.db
}

// Queries returns the underlying sqlc Queries for direct access.
func (s *Store) Queries() *Queries {
	return s.q
}

// RecordRun inserts a new run and returns its ID.
func (s *Store) RecordRun(ctx context.Context, filePath string, environment string) (int64, error) {
	env := sql.NullString{String: environment, Valid: environment != ""}
	run, err := s.q.InsertRun(ctx, InsertRunParams{
		FilePath:    filePath,
		Environment: env,
	})
	if err != nil {
		return 0, fmt.Errorf("history: insert run: %w", err)
	}
	return run.ID, nil
}

// FinishRun updates a run with its final results.
func (s *Store) FinishRun(ctx context.Context, id int64, duration time.Duration, passed, failed, skipped, total int64) error {
	now := sql.NullTime{Time: time.Now(), Valid: true}
	return s.q.UpdateRunFinished(ctx, UpdateRunFinishedParams{
		ID:         id,
		FinishedAt: now,
		DurationMs: duration.Milliseconds(),
		Passed:     passed,
		Failed:     failed,
		Skipped:    skipped,
		Total:      total,
	})
}

// RecordResult inserts a single request result tied to a run and returns its ID.
func (s *Store) RecordResult(ctx context.Context, runID int64, name, method, url string, statusCode int, durationMs int64, passed, skipped bool, errMsg, description, bodyPreview string) (int64, error) {
	result, err := s.q.InsertResult(ctx, InsertResultParams{
		RunID:       runID,
		RequestName: name,
		Method:      method,
		Url:         url,
		StatusCode:  nullInt64(int64(statusCode)),
		DurationMs:  durationMs,
		Passed:      passed,
		Skipped:     skipped,
		Error:       nullString(errMsg),
		Description: nullString(description),
		BodyPreview: nullString(bodyPreview),
	})
	if err != nil {
		return 0, fmt.Errorf("history: insert result: %w", err)
	}
	return result.ID, nil
}

// RecordAssertions inserts a batch of assertions for a given result.
func (s *Store) RecordAssertions(ctx context.Context, resultID int64, assertions []AssertionRecord) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("history: begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	qtx := s.q.WithTx(tx)
	for _, a := range assertions {
		if err := qtx.InsertAssertion(ctx, InsertAssertionParams{
			ResultID: resultID,
			Operator: a.Operator,
			Subject:  a.Subject,
			Expected: nullString(a.Expected),
			Actual:   nullString(a.Actual),
			Passed:   a.Passed,
			Message:  nullString(a.Message),
		}); err != nil {
			return fmt.Errorf("history: insert assertion: %w", err)
		}
	}

	return tx.Commit()
}

// AssertionRecord is the input struct for recording a single assertion.
type AssertionRecord struct {
	Operator string
	Subject  string
	Expected string
	Actual   string
	Passed   bool
	Message  string
}

func nullString(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}

func nullInt64(n int64) sql.NullInt64 {
	return sql.NullInt64{Int64: n, Valid: n != 0}
}

// ResultRecord is one request result of a run, with its assertions.
type ResultRecord struct {
	Name        string
	Method      string
	URL         string
	StatusCode  int
	DurationMs  int64
	Passed      bool
	Skipped     bool
	Error       string
	Description string
	BodyPreview string
	Assertions  []AssertionRecord
}

// RunRecord is a finished run, ready to be stored in one transaction.
type RunRecord struct {
	FilePath    string
	Environment string
	Duration    time.Duration
	Results     []ResultRecord
}

// RecordCompleteRun stores a finished run, its results and their assertions in
// a single transaction and returns the run ID. Either everything is committed
// or nothing is, so a reader that sees the run sees all of it (a run is never
// listed with a partial or empty result set). The pass/fail/skip counts are
// derived from the results.
func (s *Store) RecordCompleteRun(ctx context.Context, rec RunRecord) (int64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("history: begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op after Commit

	qtx := s.q.WithTx(tx)
	run, err := qtx.InsertRun(ctx, InsertRunParams{
		FilePath:    rec.FilePath,
		Environment: sql.NullString{String: rec.Environment, Valid: rec.Environment != ""},
	})
	if err != nil {
		return 0, fmt.Errorf("history: insert run: %w", err)
	}

	var passed, failed, skipped int64
	for _, r := range rec.Results {
		res, err := qtx.InsertResult(ctx, InsertResultParams{
			RunID:       run.ID,
			RequestName: r.Name,
			Method:      r.Method,
			Url:         r.URL,
			StatusCode:  nullInt64(int64(r.StatusCode)),
			DurationMs:  r.DurationMs,
			Passed:      r.Passed,
			Skipped:     r.Skipped,
			Error:       nullString(r.Error),
			Description: nullString(r.Description),
			BodyPreview: nullString(r.BodyPreview),
		})
		if err != nil {
			return 0, fmt.Errorf("history: insert result: %w", err)
		}
		for _, a := range r.Assertions {
			if err := qtx.InsertAssertion(ctx, InsertAssertionParams{
				ResultID: res.ID,
				Operator: a.Operator,
				Subject:  a.Subject,
				Expected: nullString(a.Expected),
				Actual:   nullString(a.Actual),
				Passed:   a.Passed,
				Message:  nullString(a.Message),
			}); err != nil {
				return 0, fmt.Errorf("history: insert assertion: %w", err)
			}
		}
		switch {
		case r.Passed:
			passed++
		case r.Skipped:
			skipped++
		default:
			failed++
		}
	}

	if err := qtx.UpdateRunFinished(ctx, UpdateRunFinishedParams{
		ID:         run.ID,
		FinishedAt: sql.NullTime{Time: time.Now(), Valid: true},
		DurationMs: rec.Duration.Milliseconds(),
		Passed:     passed,
		Failed:     failed,
		Skipped:    skipped,
		Total:      passed + failed + skipped,
	}); err != nil {
		return 0, fmt.Errorf("history: finish run: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("history: commit: %w", err)
	}
	return run.ID, nil
}
