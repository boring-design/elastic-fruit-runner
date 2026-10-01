-- +goose Up
CREATE TABLE jobs (
    id              TEXT PRIMARY KEY,
    runner_name     TEXT NOT NULL DEFAULT '',
    runner_set_name TEXT NOT NULL DEFAULT '',
    result          TEXT NOT NULL DEFAULT 'running',
    started_at      DATETIME NOT NULL,
    completed_at    DATETIME
);

CREATE INDEX idx_jobs_started_at ON jobs (started_at DESC);

-- +goose Down
DROP TABLE jobs;
