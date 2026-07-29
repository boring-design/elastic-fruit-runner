package management

import (
	"context"
	"database/sql"
	"log/slog"
	"time"

	"github.com/boring-design/elastic-fruit-runner/internal/controller"
	sqlcdb "github.com/boring-design/elastic-fruit-runner/internal/management/sqlc"
)

// JobRecord represents a single job execution.
type JobRecord struct {
	ID            string
	RunnerName    string
	RunnerSetName string
	Result        string
	StartedAt     time.Time
	CompletedAt   *time.Time
	// Repository is the GitHub repository the job belongs to in "owner/repo"
	// form. Captured at job-start time; empty if the start event was missed.
	Repository string
	// WorkflowName is the human-readable workflow name from the GitHub
	// JobMessageBase.JobDisplayName field (best-effort; may be empty).
	WorkflowName string
	// WorkflowRunID is the GitHub Actions workflow run identifier. Combined
	// with Repository, callers can build an Actions URL.
	WorkflowRunID string
}

// JobStore persists job lifecycle events in SQLite.
type JobStore struct {
	queries *sqlcdb.Queries
}

// NewJobStore creates a SQLite-backed job store.
func NewJobStore(db *sql.DB) *JobStore {
	return &JobStore{
		queries: sqlcdb.New(db),
	}
}

// knownJobResults is the set of valid result strings from the GitHub Scale Set API.
var knownJobResults = map[string]struct{}{
	"succeeded": {},
	"failed":    {},
	"canceled":  {},
}

// RecordJobStarted inserts a new job with result "running" along with the
// context metadata captured at start time (repository, workflow name, run ID).
func (s *JobStore) RecordJobStarted(start controller.JobStart) {
	ctx := context.Background()
	err := s.queries.InsertJob(ctx, sqlcdb.InsertJobParams{
		ID:            start.JobID,
		RunnerName:    start.RunnerName,
		RunnerSetName: start.RunnerSetName,
		Result:        "running",
		StartedAt:     time.Now(),
		Repository:    start.Repository,
		WorkflowName:  start.WorkflowName,
		WorkflowRunID: start.WorkflowRunID,
	})
	if err != nil {
		slog.Error("failed to record job started",
			"job_id", start.JobID,
			"runner_set", start.RunnerSetName,
			"repository", start.Repository,
			"err", err,
		)
	}
}

// RecordJobCompleted finds an existing job by ID and updates its result.
// If the job does not exist, inserts a completed-only record.
func (s *JobStore) RecordJobCompleted(jobID, result string) {
	if _, ok := knownJobResults[result]; !ok {
		slog.Error("unexpected job result from scale-set API, refusing to record invalid state", "job_id", jobID, "result", result)
		return
	}

	ctx := context.Background()
	now := time.Now()

	res, err := s.queries.UpdateJobCompleted(ctx, sqlcdb.UpdateJobCompletedParams{
		Result:      result,
		CompletedAt: sql.NullTime{Time: now, Valid: true},
		ID:          jobID,
	})
	if err != nil {
		slog.Error("failed to update job completed", "job_id", jobID, "err", err)
		return
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		slog.Error("failed to check rows affected", "job_id", jobID, "err", err)
		return
	}

	if rowsAffected == 0 {
		// Job was never recorded (e.g. daemon restarted). Insert a completed-only record.
		err = s.queries.InsertCompletedJob(ctx, sqlcdb.InsertCompletedJobParams{
			ID:          jobID,
			Result:      result,
			StartedAt:   now,
			CompletedAt: sql.NullTime{Time: now, Valid: true},
		})
		if err != nil {
			slog.Error("failed to insert completed-only job record", "job_id", jobID, "err", err)
		}
	}
}

// Snapshot returns recent job records, most-recent-first.
func (s *JobStore) Snapshot() []JobRecord {
	ctx := context.Background()
	rows, err := s.queries.ListRecentJobs(ctx, 200)
	if err != nil {
		slog.Error("failed to list recent jobs", "err", err)
		return nil
	}

	records := make([]JobRecord, len(rows))
	for i, row := range rows {
		records[i] = JobRecord{
			ID:            row.ID,
			RunnerName:    row.RunnerName,
			RunnerSetName: row.RunnerSetName,
			Result:        row.Result,
			StartedAt:     row.StartedAt,
			Repository:    row.Repository,
			WorkflowName:  row.WorkflowName,
			WorkflowRunID: row.WorkflowRunID,
		}
		if row.CompletedAt.Valid {
			t := row.CompletedAt.Time
			records[i].CompletedAt = &t
		}
	}
	return records
}
