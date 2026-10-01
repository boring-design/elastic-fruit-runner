// Package storage opens the single SQLite database shared by every service
// and keeps its schema up to date.
package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pressly/goose/v3"
	// Register the pure Go SQLite driver. This is the only place in the repo that imports it.
	_ "modernc.org/sqlite"

	"github.com/boring-design/elastic-fruit-runner/internal/storage/migrations"
)

// Open creates the database file when needed, applies pending migrations,
// and returns a connection ready for use. Pass ":memory:" for a throwaway database.
func Open(path string) (*sql.DB, error) {
	if path == "" {
		return nil, errors.New("open database: database path is empty")
	}
	if path != ":memory:" {
		dir := filepath.Dir(path)
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return nil, fmt.Errorf("create database directory %s: %w", dir, err)
		}
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open database %s: %w", path, err)
	}
	// SQLite does not benefit from multiple connections. A single connection
	// avoids "database is locked" errors and works correctly with :memory:.
	db.SetMaxOpenConns(1)

	ctx := context.Background()
	if _, err := db.ExecContext(ctx, "PRAGMA busy_timeout=5000"); err != nil {
		db.Close()
		return nil, fmt.Errorf("set busy timeout on database %s: %w", path, err)
	}
	if _, err := db.ExecContext(ctx, "PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("enable WAL mode on database %s: %w", path, err)
	}

	provider, err := goose.NewProvider(goose.DialectSQLite3, db, migrations.FS)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("prepare migrations for database %s: %w", path, err)
	}
	if _, err := provider.Up(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("run migrations on database %s: %w", path, err)
	}

	if path != ":memory:" {
		if err := os.Chmod(path, 0o600); err != nil {
			db.Close()
			return nil, fmt.Errorf("set database permissions %s: %w", path, err)
		}
	}
	return db, nil
}

// Compact moves WAL content back into the main file and reclaims free pages.
func Compact(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		return fmt.Errorf("checkpoint database WAL: %w", err)
	}
	if _, err := db.ExecContext(ctx, "VACUUM"); err != nil {
		return fmt.Errorf("vacuum database: %w", err)
	}
	return nil
}

// FileSize returns the total bytes used by the database file and its WAL companions.
func FileSize(path string) int64 {
	if path == "" || path == ":memory:" {
		return 0
	}
	var size int64
	for _, suffix := range []string{"", "-wal", "-shm"} {
		info, err := os.Stat(filepath.Clean(path + suffix))
		if err == nil {
			size += info.Size()
		}
	}
	return size
}
