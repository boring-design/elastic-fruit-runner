package management

import (
	"database/sql"
	"fmt"
	"sync"
	"testing"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"

	"github.com/boring-design/elastic-fruit-runner/internal/management/migrations"
)

func setupTestStore(t *testing.T) *JobStore {
	t.Helper()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open in-memory sqlite: %v", err)
	}
	db.SetMaxOpenConns(1)

	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("sqlite3"); err != nil {
		t.Fatalf("set goose dialect: %v", err)
	}
	if err := goose.Up(db, "."); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	t.Cleanup(func() { db.Close() })
	return NewJobStore(db)
}

func TestRecordJobStarted(t *testing.T) {
	store := setupTestStore(t)

	store.RecordJobStarted("set-a", "job-1", "runner-1")

	jobs := store.Snapshot()
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
	if jobs[0].ID != "job-1" {
		t.Errorf("expected job ID job-1, got %s", jobs[0].ID)
	}
	if jobs[0].RunnerName != "runner-1" {
		t.Errorf("expected runner name runner-1, got %s", jobs[0].RunnerName)
	}
	if jobs[0].RunnerSetName != "set-a" {
		t.Errorf("expected runner set name set-a, got %s", jobs[0].RunnerSetName)
	}
	if jobs[0].Result != "running" {
		t.Errorf("expected result running, got %s", jobs[0].Result)
	}
	if jobs[0].CompletedAt != nil {
		t.Errorf("expected CompletedAt to be nil")
	}
}

func TestRecordJobCompleted(t *testing.T) {
	store := setupTestStore(t)

	store.RecordJobStarted("set-a", "job-1", "runner-1")
	store.RecordJobCompleted("job-1", "Succeeded")

	jobs := store.Snapshot()
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
	if jobs[0].Result != "Succeeded" {
		t.Errorf("expected result Succeeded, got %s", jobs[0].Result)
	}
	if jobs[0].CompletedAt == nil {
		t.Fatalf("expected CompletedAt to be set")
	}
}

func TestRecordJobCompletedUnknownJob(t *testing.T) {
	store := setupTestStore(t)

	// Complete a job that was never started (e.g. daemon restarted)
	store.RecordJobCompleted("orphan-job", "Failed")

	jobs := store.Snapshot()
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
	if jobs[0].ID != "orphan-job" {
		t.Errorf("expected job ID orphan-job, got %s", jobs[0].ID)
	}
	if jobs[0].Result != "Failed" {
		t.Errorf("expected result Failed, got %s", jobs[0].Result)
	}
	if jobs[0].RunnerName != "" {
		t.Errorf("expected empty runner name for orphan job, got %s", jobs[0].RunnerName)
	}
}

func TestSnapshotOrdering(t *testing.T) {
	store := setupTestStore(t)

	store.RecordJobStarted("set-a", "job-1", "runner-1")
	store.RecordJobStarted("set-a", "job-2", "runner-2")
	store.RecordJobStarted("set-a", "job-3", "runner-3")

	jobs := store.Snapshot()
	if len(jobs) != 3 {
		t.Fatalf("expected 3 jobs, got %d", len(jobs))
	}
	// Most recent first
	if jobs[0].ID != "job-3" {
		t.Errorf("expected first job to be job-3, got %s", jobs[0].ID)
	}
	if jobs[1].ID != "job-2" {
		t.Errorf("expected second job to be job-2, got %s", jobs[1].ID)
	}
	if jobs[2].ID != "job-1" {
		t.Errorf("expected third job to be job-1, got %s", jobs[2].ID)
	}
}

func TestSnapshotLimit(t *testing.T) {
	store := setupTestStore(t)

	for i := range 250 {
		store.RecordJobStarted("set-a", fmt.Sprintf("job-%d", i), fmt.Sprintf("runner-%d", i))
	}

	jobs := store.Snapshot()
	if len(jobs) != 200 {
		t.Errorf("expected 200 jobs (limit), got %d", len(jobs))
	}
}

func TestUnexpectedResult(t *testing.T) {
	store := setupTestStore(t)

	store.RecordJobStarted("set-a", "job-1", "runner-1")
	store.RecordJobCompleted("job-1", "Cancelled")

	jobs := store.Snapshot()
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
	if jobs[0].Result != "Cancelled" {
		t.Errorf("expected result Cancelled, got %s", jobs[0].Result)
	}
}

func TestConcurrentAccess(t *testing.T) {
	store := setupTestStore(t)

	var wg sync.WaitGroup
	for i := range 50 {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			id := fmt.Sprintf("job-%d", n)
			store.RecordJobStarted("set-a", id, fmt.Sprintf("runner-%d", n))
			store.RecordJobCompleted(id, "Succeeded")
		}(i)
	}
	wg.Wait()

	jobs := store.Snapshot()
	if len(jobs) != 50 {
		t.Errorf("expected 50 jobs, got %d", len(jobs))
	}
}
