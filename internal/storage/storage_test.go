package storage

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func TestOpen_CreatesDirectoryAndMigrates(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "nested", "deeper", "test.db")

	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open(%q) error: %v", path, err)
	}
	defer db.Close()

	for _, table := range []string{
		"jobs", "job_logs", "job_resource_samples", "host_resource_samples",
		"console_admin", "console_sessions", "config_revisions", "goose_db_version",
	} {
		if !tableExists(t, db, table) {
			t.Errorf("table %q missing after Open", table)
		}
	}
}

func TestOpen_AdoptsTablesCreatedBeforeGoose(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "old.db")

	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("sql.Open(%q) error: %v", path, err)
	}
	// This is the exact DDL the auth and configstate packages used to run on their own.
	oldDDL := `
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
		CREATE INDEX IF NOT EXISTS idx_console_sessions_expires_at
		ON console_sessions (expires_at);
		CREATE TABLE IF NOT EXISTS config_revisions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			created_at DATETIME NOT NULL,
			source TEXT NOT NULL,
			config_hash TEXT NOT NULL,
			config_yaml BLOB NOT NULL,
			active INTEGER NOT NULL DEFAULT 0
		);
		CREATE INDEX IF NOT EXISTS idx_config_revisions_created_at
		ON config_revisions (created_at DESC);
	`
	if _, err := raw.Exec(oldDDL); err != nil {
		t.Fatalf("create old tables: %v", err)
	}
	if _, err := raw.Exec(`INSERT INTO console_admin (id, password_hash, created_at) VALUES (1, X'ABCD', 1700000000)`); err != nil {
		t.Fatalf("insert old admin row: %v", err)
	}
	if _, err := raw.Exec(`INSERT INTO config_revisions (created_at, source, config_hash, config_yaml, active) VALUES ('2024-01-01 00:00:00', 'startup', 'hash', X'00', 1)`); err != nil {
		t.Fatalf("insert old config revision: %v", err)
	}
	if err := raw.Close(); err != nil {
		t.Fatalf("close raw database: %v", err)
	}

	for attempt := 1; attempt <= 2; attempt++ {
		db, err := Open(path)
		if err != nil {
			t.Fatalf("Open(%q) attempt %d error: %v", path, attempt, err)
		}

		var version int64
		if err := db.QueryRow(`SELECT MAX(version_id) FROM goose_db_version`).Scan(&version); err != nil {
			t.Fatalf("attempt %d: read goose version: %v", attempt, err)
		}
		if version != 3 {
			t.Errorf("attempt %d: goose version = %d, want 3", attempt, version)
		}
		if !tableExists(t, db, "jobs") {
			t.Errorf("attempt %d: jobs table missing", attempt)
		}
		var adminCount, revisionCount int64
		if err := db.QueryRow(`SELECT COUNT(*) FROM console_admin`).Scan(&adminCount); err != nil {
			t.Fatalf("attempt %d: count console_admin: %v", attempt, err)
		}
		if err := db.QueryRow(`SELECT COUNT(*) FROM config_revisions WHERE active = 1`).Scan(&revisionCount); err != nil {
			t.Fatalf("attempt %d: count config_revisions: %v", attempt, err)
		}
		if adminCount != 1 || revisionCount != 1 {
			t.Errorf("attempt %d: rows lost, admin=%d revisions=%d, want 1 and 1", attempt, adminCount, revisionCount)
		}
		if err := db.Close(); err != nil {
			t.Fatalf("attempt %d: close database: %v", attempt, err)
		}
	}
}

func TestOpen_RejectsEmptyPath(t *testing.T) {
	t.Parallel()
	if _, err := Open(""); err == nil {
		t.Fatal("Open(\"\") returned nil error, want error")
	}
}

func tableExists(t *testing.T, db *sql.DB, name string) bool {
	t.Helper()
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, name).Scan(&count); err != nil {
		t.Fatalf("query sqlite_master for %q: %v", name, err)
	}
	return count == 1
}
