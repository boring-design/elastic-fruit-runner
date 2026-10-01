-- +goose Up
CREATE TABLE IF NOT EXISTS console_admin (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    password_hash BLOB NOT NULL,
    created_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS console_sessions (
    token_hash TEXT PRIMARY KEY,
    csrf_token TEXT NOT NULL,
    expires_at INTEGER NOT NULL,
    created_at INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_console_sessions_expires_at ON console_sessions (expires_at);

CREATE TABLE IF NOT EXISTS config_revisions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    created_at DATETIME NOT NULL,
    source TEXT NOT NULL,
    config_hash TEXT NOT NULL,
    config_yaml BLOB NOT NULL,
    active INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_config_revisions_created_at ON config_revisions (created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS config_revisions;
DROP TABLE IF EXISTS console_sessions;
DROP TABLE IF EXISTS console_admin;
