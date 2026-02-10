CREATE TABLE IF NOT EXISTS runs (
    id          INTEGER PRIMARY KEY,
    file_path   TEXT     NOT NULL,
    environment TEXT,
    started_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    finished_at DATETIME,
    duration_ms INTEGER  NOT NULL DEFAULT 0,
    passed      INTEGER  NOT NULL DEFAULT 0,
    failed      INTEGER  NOT NULL DEFAULT 0,
    skipped     INTEGER  NOT NULL DEFAULT 0,
    total       INTEGER  NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_runs_started_at ON runs(started_at);

CREATE TABLE IF NOT EXISTS results (
    id          INTEGER  PRIMARY KEY,
    run_id      INTEGER  NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    request_name TEXT    NOT NULL,
    method      TEXT     NOT NULL,
    url         TEXT     NOT NULL,
    status_code INTEGER,
    duration_ms INTEGER  NOT NULL DEFAULT 0,
    passed      BOOLEAN  NOT NULL DEFAULT false,
    skipped     BOOLEAN  NOT NULL DEFAULT false,
    error       TEXT,
    description TEXT,
    body_preview TEXT,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_results_run_id ON results(run_id);

CREATE TABLE IF NOT EXISTS assertions (
    id        INTEGER PRIMARY KEY,
    result_id INTEGER NOT NULL REFERENCES results(id) ON DELETE CASCADE,
    operator  TEXT    NOT NULL,
    subject   TEXT    NOT NULL,
    expected  TEXT,
    actual    TEXT,
    passed    BOOLEAN NOT NULL DEFAULT false,
    message   TEXT
);

CREATE INDEX IF NOT EXISTS idx_assertions_result_id ON assertions(result_id);
