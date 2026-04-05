package management

import (
	"context"
	"database/sql"
	"log/slog"
	"time"

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

// knownJobResults is the set of valid result strings from the GitHub Actions API.
var knownJobResults = map[string]struct{}{
	"Succeeded": {},
	"Failed":    {},
}

// RecordJobStarted inserts a new job with result "running".
func (s *JobStore) RecordJobStarted(setName, jobID, runnerName string) {
	ctx := context.Background()
	err := s.queries.InsertJob(ctx, sqlcdb.InsertJobParams{
		ID:            jobID,
		RunnerName:    runnerName,
		RunnerSetName: setName,
		Result:        "running",
		StartedAt:     time.Now(),
	})
	if err != nil {
		slog.Error("failed to record job started", "job_id", jobID, "err", err)
	}
}

// RecordJobCompleted finds an existing job by ID and updates its result.
// If the job does not exist, inserts a completed-only record.
func (s *JobStore) RecordJobCompleted(jobID, result string) {
	if _, ok := knownJobResults[result]; !ok {
		slog.Warn("unexpected job result from scale-set API, recording as-is", "job_id", jobID, "result", result)
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
		}
		if row.CompletedAt.Valid {
			t := row.CompletedAt.Time
			records[i].CompletedAt = &t
		}
	}
	return records
}
