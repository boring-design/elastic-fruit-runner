package management

import (
	"context"
	"database/sql"
	"log/slog"
	"strings"
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

// knownJobResults is the set of valid result strings the GitHub Actions Scale
// Set API is documented to send. Comparison is case-insensitive (the API has
// historically sent both lowercase and Title Case forms across versions).
var knownJobResults = map[string]struct{}{
	"succeeded": {},
	"failed":    {},
	"canceled":  {},
}

// isKnownJobResult reports whether result is one of the documented Scale Set
// API result strings (case-insensitive).
func isKnownJobResult(result string) bool {
	_, ok := knownJobResults[strings.ToLower(result)]
	return ok
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
		slog.Error("failed to record job started",
			"job_id", jobID,
			"runner_name", runnerName,
			"runner_set_name", setName,
			"err", err,
		)
	}
}

// RecordJobCompleted finds an existing job by ID and updates its result.
// If the job does not exist (e.g. the daemon restarted between start and
// completion events), inserts a completed-only record using the runner name
// and set name from the completion event so the dashboard can still display
// the job correctly.
//
// Unknown result strings are persisted as-is (with a warning) so the row
// transitions out of "running" and the COMPLETED counter increments. The
// API layer maps unknown values to JOB_RESULT_UNKNOWN for the dashboard.
func (s *JobStore) RecordJobCompleted(setName, jobID, runnerName, result string) {
	if !isKnownJobResult(result) {
		slog.Warn("unrecognised job result from scale-set API; recording as-is",
			"job_id", jobID,
			"runner_name", runnerName,
			"runner_set_name", setName,
			"result", result,
		)
	}

	ctx := context.Background()
	now := time.Now()

	res, err := s.queries.UpdateJobCompleted(ctx, sqlcdb.UpdateJobCompletedParams{
		Result:      result,
		CompletedAt: sql.NullTime{Time: now, Valid: true},
		ID:          jobID,
	})
	if err != nil {
		slog.Error("failed to update job completed",
			"job_id", jobID,
			"runner_name", runnerName,
			"runner_set_name", setName,
			"result", result,
			"err", err,
		)
		return
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		slog.Error("failed to check rows affected for completion update",
			"job_id", jobID,
			"runner_name", runnerName,
			"runner_set_name", setName,
			"err", err,
		)
		return
	}

	if rowsAffected > 0 {
		return
	}

	// Job was never recorded as started (e.g. daemon restarted, or the start
	// event was missed). Insert a completed-only record carrying the runner
	// names from the completion event so downstream consumers have full
	// context — empty fields here previously crashed the dashboard.
	err = s.queries.InsertCompletedJob(ctx, sqlcdb.InsertCompletedJobParams{
		ID:            jobID,
		RunnerName:    runnerName,
		RunnerSetName: setName,
		Result:        result,
		StartedAt:     now,
		CompletedAt:   sql.NullTime{Time: now, Valid: true},
	})
	if err != nil {
		slog.Error("failed to insert completed-only job record",
			"job_id", jobID,
			"runner_name", runnerName,
			"runner_set_name", setName,
			"result", result,
			"err", err,
		)
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
