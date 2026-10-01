package management

import (
	"path/filepath"
	"testing"

	"github.com/boring-design/elastic-fruit-runner/config"
	"github.com/boring-design/elastic-fruit-runner/internal/backend"
	"github.com/boring-design/elastic-fruit-runner/internal/storage"
)

func TestJobStore_RecordsStartAndCompletion(t *testing.T) {
	t.Parallel()
	dbPath := filepath.Join(t.TempDir(), "test.db")

	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("storage.Open(%q) error: %v", dbPath, err)
	}
	defer db.Close()

	store := NewJobStore(db)

	store.RecordJobStarted("set-1", "job-1", "runner-1")
	store.RecordJobCompleted("job-1", "succeeded")

	jobs := store.Snapshot()
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
	if jobs[0].ID != "job-1" {
		t.Errorf("job ID = %q, want %q", jobs[0].ID, "job-1")
	}
	if jobs[0].Result != "succeeded" {
		t.Errorf("job result = %q, want %q", jobs[0].Result, "succeeded")
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
