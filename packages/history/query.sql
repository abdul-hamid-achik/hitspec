-- name: InsertRun :one
INSERT INTO runs (file_path, environment)
VALUES (?, ?)
RETURNING *;

-- name: UpdateRunFinished :exec
UPDATE runs
SET finished_at = ?, duration_ms = ?, passed = ?, failed = ?, skipped = ?, total = ?
WHERE id = ?;

-- name: InsertResult :one
INSERT INTO results (run_id, request_name, method, url, status_code, duration_ms, passed, skipped, error, description, body_preview)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: InsertAssertion :exec
INSERT INTO assertions (result_id, operator, subject, expected, actual, passed, message)
VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: GetRun :one
SELECT * FROM runs WHERE id = ?;

-- name: ListRuns :many
SELECT * FROM runs ORDER BY started_at DESC LIMIT ? OFFSET ?;

-- name: CountRuns :one
SELECT COUNT(*) FROM runs;

-- name: ListResultsByRun :many
SELECT * FROM results WHERE run_id = ? ORDER BY id;

-- name: ListAssertionsByResult :many
SELECT * FROM assertions WHERE result_id = ? ORDER BY id;

-- name: DeleteRun :exec
DELETE FROM runs WHERE id = ?;

-- name: DeleteRunsBefore :exec
DELETE FROM runs WHERE started_at < ?;

-- name: ClearAllRuns :exec
DELETE FROM runs;
