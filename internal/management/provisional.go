package management

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/actions/scaleset"

	"github.com/boring-design/elastic-fruit-runner/internal/backend"
	sqlcdb "github.com/boring-design/elastic-fruit-runner/internal/storage/sqlc"
)

// ProvisionalDisplayName is shown for a job record opened before the job
// details are known.
const ProvisionalDisplayName = "waiting for job"

// runnerJobStartMarker is the line the GitHub runner prints when it picks up
// a job, so the captured runner log tells whether a job ran at all.
const runnerJobStartMarker = "Running job:"

// OpenProvisionalJob records a running job for a runner before the job
// details are known and starts log and sample capture right away. The record
// uses a placeholder id chosen by the caller until AttachJobMetadata moves it
// to the real job id.
func (s *JobStore) OpenProvisionalJob(placeholderID, setName, backendName, runnerName string, diagnostics backend.Diagnostics) {
	s.RecordJobMessageStarted(setName, backendName, diagnostics, &scaleset.JobStarted{
		RunnerName: runnerName,
		JobMessageBase: scaleset.JobMessageBase{
			JobID:          placeholderID,
			JobDisplayName: ProvisionalDisplayName,
		},
	})
}

// AttachJobMetadata moves the provisional record under placeholderID to the
// real job id and fills in the job details. Logs and samples captured so far
// move with it and capture keeps running under the new id. It returns false
// when there is no running provisional record, then nothing was changed.
func (s *JobStore) AttachJobMetadata(placeholderID, setName, backendName string, diagnostics backend.Diagnostics, job *scaleset.JobStarted) bool {
	ctx := context.Background()
	row, err := s.queries.GetJob(ctx, placeholderID)
	if errors.Is(err, sql.ErrNoRows) {
		return false
	}
	if err != nil {
		slog.Error("failed to read provisional job record", "placeholder_id", placeholderID, "job_id", job.JobID, "runner", job.RunnerName, "err", err)
		return false
	}
	if row.Result != "running" {
		return false
	}
	mergedSetName, mergedBackend, merged := mergeJobMetadata(jobRecordFromRow(row), setName, backendName, job)

	state := s.lookupCapture(placeholderID)
	if state != nil {
		state.mu.Lock()
		defer state.mu.Unlock()
	}
	if err := s.moveJob(ctx, placeholderID, startedJobParams(mergedSetName, mergedBackend, merged)); err != nil {
		slog.Error("failed to attach job details to provisional record", "placeholder_id", placeholderID, "job_id", job.JobID, "runner", job.RunnerName, "err", err)
		return false
	}
	switch {
	case state != nil:
		state.jobID = merged.JobID
		s.captureMu.Lock()
		delete(s.captures, placeholderID)
		s.captures[merged.JobID] = state
		s.captureMu.Unlock()
	case diagnostics != nil:
		s.startCapture(merged.JobID, merged.RunnerName, diagnostics)
	}
	return true
}

// CloseProvisionalJob ends the provisional record under placeholderID when
// its runner is removed without the control plane ever naming the job. The
// result is "unknown" and the display name says whether a job ran at all.
// It does nothing when the record was already attached to a real job.
func (s *JobStore) CloseProvisionalJob(placeholderID, runnerName string) {
	s.stopCapture(placeholderID)
	ctx := context.Background()
	row, err := s.queries.GetJob(ctx, placeholderID)
	if errors.Is(err, sql.ErrNoRows) {
		return
	}
	if err != nil {
		slog.Error("failed to read provisional job record", "placeholder_id", placeholderID, "runner", runnerName, "err", err)
		return
	}
	if row.Result != "running" {
		return
	}
	note := closedWithoutJobNote(logTextShowsJobStarted(s.allLogText(placeholderID)))
	err = s.queries.CloseRunningJobWithNote(ctx, sqlcdb.CloseRunningJobWithNoteParams{
		Result:      "unknown",
		CompletedAt: sql.NullTime{Time: time.Now(), Valid: true},
		DisplayName: note,
		ID:          placeholderID,
	})
	if err != nil {
		slog.Error("failed to close provisional job record", "placeholder_id", placeholderID, "runner", runnerName, "err", err)
	}
}

// mergeJobMetadata combines the real job details with a provisional record.
// Values the control plane sent win. Set name, backend, runner name, and
// display name fall back to what the agent already knew when they come empty.
func mergeJobMetadata(existing JobRecord, setName, backendName string, incoming *scaleset.JobStarted) (mergedSetName, mergedBackend string, merged *scaleset.JobStarted) {
	copied := *incoming
	if copied.RunnerName == "" {
		copied.RunnerName = existing.RunnerName
	}
	if copied.JobDisplayName == "" {
		copied.JobDisplayName = existing.DisplayName
	}
	return firstNonEmpty(setName, existing.RunnerSetName), firstNonEmpty(backendName, existing.Backend), &copied
}

// closedWithoutJobNote is the display name for a provisional record that was
// closed because its runner went away before any job details arrived.
func closedWithoutJobNote(jobRan bool) string {
	if jobRan {
		return "job ran while the control plane was unreachable, details unknown"
	}
	return "runner removed before any job was assigned"
}

// logTextShowsJobStarted reports whether the runner log says a job started.
func logTextShowsJobStarted(text string) bool {
	return strings.Contains(text, runnerJobStartMarker)
}

// moveJob renames a running job row to the id in params, writes the merged
// details, and moves its logs and samples, all in one transaction.
func (s *JobStore) moveJob(ctx context.Context, oldID string, params sqlcdb.UpsertStartedJobParams) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()
	queries := s.queries.WithTx(tx)
	result, err := queries.RenameRunningJob(ctx, sqlcdb.RenameRunningJobParams{NewID: params.ID, OldID: oldID})
	if err != nil {
		return fmt.Errorf("rename job %s to %s: %w", oldID, params.ID, err)
	}
	renamed, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check rename of job %s to %s: %w", oldID, params.ID, err)
	}
	if renamed == 0 {
		return fmt.Errorf("job %s is not running anymore, nothing to rename to %s", oldID, params.ID)
	}
	if err := queries.UpsertStartedJob(ctx, params); err != nil {
		return fmt.Errorf("write details of job %s: %w", params.ID, err)
	}
	if err := queries.MoveJobLogs(ctx, sqlcdb.MoveJobLogsParams{NewID: params.ID, OldID: oldID}); err != nil {
		return fmt.Errorf("move logs of job %s to %s: %w", oldID, params.ID, err)
	}
	if err := queries.MoveJobResourceSamples(ctx, sqlcdb.MoveJobResourceSamplesParams{NewID: params.ID, OldID: oldID}); err != nil {
		return fmt.Errorf("move samples of job %s to %s: %w", oldID, params.ID, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit move of job %s to %s: %w", oldID, params.ID, err)
	}
	return nil
}

func (s *JobStore) lookupCapture(jobID string) *captureState {
	s.captureMu.Lock()
	defer s.captureMu.Unlock()
	return s.captures[jobID]
}

// allLogText joins every stored log chunk of a job.
func (s *JobStore) allLogText(jobID string) string {
	var text strings.Builder
	var after int64
	for {
		logs, next := s.Logs(jobID, after, 500)
		for _, entry := range logs {
			text.WriteString(entry.Text)
		}
		if len(logs) == 0 || next == after {
			return text.String()
		}
		after = next
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
