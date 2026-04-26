package management

import (
	"path/filepath"
	"testing"
)

func TestRecordJobCompleted_PassesThroughRunnerNames_OrphanRecord(t *testing.T) {
	t.Parallel()
	dbPath := filepath.Join(t.TempDir(), "test.db")

	db, err := openJobsDB(dbPath)
	if err != nil {
		t.Fatalf("openJobsDB error: %v", err)
	}
	defer db.Close()

	store := NewJobStore(db)

	// Simulate orphan: completion event arrives without prior start (e.g. daemon
	// restart). The runner name and set must still be persisted so the dashboard
	// can display the job correctly.
	store.RecordJobCompleted("set-x", "job-orphan", "runner-orphan", "succeeded")

	jobs := store.Snapshot()
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
	got := jobs[0]
	if got.ID != "job-orphan" {
		t.Errorf("job ID = %q, want %q", got.ID, "job-orphan")
	}
	if got.RunnerName != "runner-orphan" {
		t.Errorf("RunnerName = %q, want %q", got.RunnerName, "runner-orphan")
	}
	if got.RunnerSetName != "set-x" {
		t.Errorf("RunnerSetName = %q, want %q", got.RunnerSetName, "set-x")
	}
	if got.Result != "succeeded" {
		t.Errorf("Result = %q, want %q", got.Result, "succeeded")
	}
	if got.CompletedAt == nil {
		t.Error("expected CompletedAt to be set on orphan completion")
	}
}

func TestRecordJobCompleted_UpdatesExistingJob(t *testing.T) {
	t.Parallel()
	dbPath := filepath.Join(t.TempDir(), "test.db")

	db, err := openJobsDB(dbPath)
	if err != nil {
		t.Fatalf("openJobsDB error: %v", err)
	}
	defer db.Close()

	store := NewJobStore(db)

	store.RecordJobStarted("set-1", "job-1", "runner-1")
	store.RecordJobCompleted("set-1", "job-1", "runner-1", "succeeded")

	jobs := store.Snapshot()
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
	got := jobs[0]
	if got.RunnerName != "runner-1" {
		t.Errorf("RunnerName = %q, want %q", got.RunnerName, "runner-1")
	}
	if got.RunnerSetName != "set-1" {
		t.Errorf("RunnerSetName = %q, want %q", got.RunnerSetName, "set-1")
	}
	if got.Result != "succeeded" {
		t.Errorf("Result = %q, want %q", got.Result, "succeeded")
	}
	if got.CompletedAt == nil {
		t.Error("expected CompletedAt to be set after completion")
	}
}

func TestRecordJobCompleted_UnknownResult_StillPersists(t *testing.T) {
	t.Parallel()
	dbPath := filepath.Join(t.TempDir(), "test.db")

	db, err := openJobsDB(dbPath)
	if err != nil {
		t.Fatalf("openJobsDB error: %v", err)
	}
	defer db.Close()

	store := NewJobStore(db)

	store.RecordJobStarted("set-1", "job-weird", "runner-weird")
	// Unknown result string from a future API change must NOT leave the job
	// stuck in the running state — it should still be marked complete so the
	// COMPLETED counter increments and the row drops out of the running set.
	store.RecordJobCompleted("set-1", "job-weird", "runner-weird", "Mysterious")

	jobs := store.Snapshot()
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
	got := jobs[0]
	if got.CompletedAt == nil {
		t.Error("expected CompletedAt to be set even for unknown result")
	}
	if got.Result == "running" {
		t.Errorf("Result = %q, expected non-running for unknown completion", got.Result)
	}
}

func TestRecordJobCompleted_AllKnownResults_RoundTrip(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		result string
	}{
		{"success", "succeeded"},
		{"failure", "failed"},
		{"canceled", "canceled"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dbPath := filepath.Join(t.TempDir(), "test.db")
			db, err := openJobsDB(dbPath)
			if err != nil {
				t.Fatalf("openJobsDB error: %v", err)
			}
			defer db.Close()

			store := NewJobStore(db)
			store.RecordJobStarted("set-1", "job-1", "runner-1")
			store.RecordJobCompleted("set-1", "job-1", "runner-1", tc.result)
			jobs := store.Snapshot()
			if len(jobs) != 1 {
				t.Fatalf("expected 1 job, got %d", len(jobs))
			}
			if jobs[0].Result != tc.result {
				t.Errorf("Result = %q, want %q", jobs[0].Result, tc.result)
			}
			if jobs[0].CompletedAt == nil {
				t.Error("expected CompletedAt to be set")
			}
		})
	}
}
