package history

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
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

// NewStore opens (or creates) a SQLite database at dbPath and applies the schema.
func NewStore(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("history: open database: %w", err)
	}

	// Enable WAL mode and foreign keys for better concurrency and referential integrity.
	for _, pragma := range []string{
		"PRAGMA journal_mode=WAL",
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

	return &Store{db: db, q: New(db)}, nil
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
