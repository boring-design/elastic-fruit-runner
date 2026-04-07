package management

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/boring-design/elastic-fruit-runner/config"
	"github.com/boring-design/elastic-fruit-runner/internal/backend"
)

func TestOpenJobsDB_CreatesAndMigrates(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "sub", "test.db")

	db, err := openJobsDB(dbPath)
	if err != nil {
		t.Fatalf("openJobsDB(%q) error: %v", dbPath, err)
	}
	defer db.Close()

	// Verify the directory was created
	if _, err := os.Stat(filepath.Join(tmpDir, "sub")); err != nil {
		t.Fatalf("expected directory to be created: %v", err)
	}

	// Verify migrations ran by checking the jobs table exists
	rows, err := db.Query("SELECT name FROM sqlite_master WHERE type='table' AND name='jobs'")
	if err != nil {
		t.Fatalf("query sqlite_master: %v", err)
	}
	defer rows.Close()
	if !rows.Next() {
		t.Fatal("expected 'jobs' table to exist after migrations")
	}
}

func TestOpenJobsDB_DefaultPath(t *testing.T) {
	// Cannot use t.Parallel() because t.Setenv modifies process environment
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	db, err := openJobsDB("")
	if err != nil {
		t.Fatalf("openJobsDB(\"\") error: %v", err)
	}
	defer db.Close()

	expectedPath := filepath.Join(tmpDir, ".elastic-fruit-runner", "jobs.db")
	if _, err := os.Stat(expectedPath); err != nil {
		t.Fatalf("expected default DB at %s: %v", expectedPath, err)
	}
}

func TestOpenJobsDB_JobStoreOperations(t *testing.T) {
	t.Parallel()
	dbPath := filepath.Join(t.TempDir(), "test.db")

	db, err := openJobsDB(dbPath)
	if err != nil {
		t.Fatalf("openJobsDB error: %v", err)
	}
	defer db.Close()

	store := NewJobStore(db)

	store.RecordJobStarted("set-1", "job-1", "runner-1")
	store.RecordJobCompleted("job-1", "Succeeded")

	jobs := store.Snapshot()
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
	if jobs[0].ID != "job-1" {
		t.Errorf("job ID = %q, want %q", jobs[0].ID, "job-1")
	}
	if jobs[0].Result != "Succeeded" {
		t.Errorf("job result = %q, want %q", jobs[0].Result, "Succeeded")
	}
	if jobs[0].CompletedAt == nil {
		t.Error("expected CompletedAt to be set")
	}
}

func TestCreateBackend_Docker(t *testing.T) {
	t.Parallel()
	b, err := createBackend(&config.RunnerSetConfig{
		Name:    "test",
		Backend: "docker",
		Image:   "ubuntu:latest",
	})
	if err != nil {
		t.Fatalf("createBackend(docker) error: %v", err)
	}
	if _, ok := b.(*backend.DockerBackend); !ok {
		t.Fatalf("expected *DockerBackend, got %T", b)
	}
}

func TestCreateBackend_Unknown(t *testing.T) {
	t.Parallel()
	_, err := createBackend(&config.RunnerSetConfig{
		Name:    "test",
		Backend: "unknown",
	})
	if err == nil {
		t.Fatal("expected error for unknown backend")
	}
}
