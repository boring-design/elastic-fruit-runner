package management

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/actions/scaleset"

	"github.com/boring-design/elastic-fruit-runner/internal/backend"
	sqlcdb "github.com/boring-design/elastic-fruit-runner/internal/management/sqlc"
)

type JobRecord struct {
	ID                 string
	RunnerName         string
	RunnerSetName      string
	Result             string
	StartedAt          time.Time
	CompletedAt        *time.Time
	Owner              string
	Repository         string
	WorkflowRef        string
	DisplayName        string
	WorkflowRunID      int64
	EventName          string
	Labels             []string
	QueuedAt           *time.Time
	ScaleSetAssignedAt *time.Time
	RunnerAssignedAt   *time.Time
	Backend            string
}

type JobFilter struct {
	Status     string
	RunnerSet  string
	Repository string
	Workflow   string
	From       *time.Time
	To         *time.Time
	Cursor     int
	PageSize   int
}

type JobPage struct {
	Records    []JobRecord
	NextCursor string
}

type JobLog struct {
	Sequence   int64
	RecordedAt time.Time
	Text       string
}

type ResourceSample = backend.ResourceSample

type captureState struct {
	cancel  context.CancelFunc
	done    chan struct{}
	logSize int
}

type JobStore struct {
	db      *sql.DB
	queries *sqlcdb.Queries

	captureMu sync.Mutex
	captures  map[string]*captureState
}

func NewJobStore(db *sql.DB) *JobStore {
	return &JobStore{
		db:       db,
		queries:  sqlcdb.New(db),
		captures: make(map[string]*captureState),
	}
}

var knownJobResults = map[string]struct{}{
	"succeeded": {},
	"failed":    {},
	"canceled":  {},
}

func (s *JobStore) RecordJobMessageStarted(setName, backendName string, diagnostics backend.Diagnostics, job *scaleset.JobStarted) {
	ctx := context.Background()
	labels, _ := json.Marshal(job.RequestLabels)
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO jobs (
			id, runner_name, runner_set_name, result, started_at, owner, repository,
			workflow_ref, display_name, workflow_run_id, event_name, labels_json,
			queued_at, scale_set_assigned_at, runner_assigned_at, backend
		) VALUES (?, ?, ?, 'running', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			runner_name = excluded.runner_name,
			runner_set_name = excluded.runner_set_name,
			result = 'running',
			owner = excluded.owner,
			repository = excluded.repository,
			workflow_ref = excluded.workflow_ref,
			display_name = excluded.display_name,
			workflow_run_id = excluded.workflow_run_id,
			event_name = excluded.event_name,
			labels_json = excluded.labels_json,
			queued_at = excluded.queued_at,
			scale_set_assigned_at = excluded.scale_set_assigned_at,
			runner_assigned_at = excluded.runner_assigned_at,
			backend = excluded.backend`,
		job.JobID,
		job.RunnerName,
		setName,
		time.Now(),
		job.OwnerName,
		job.RepositoryName,
		job.JobWorkflowRef,
		job.JobDisplayName,
		job.WorkflowRunID,
		job.EventName,
		string(labels),
		nullableTime(job.QueueTime),
		nullableTime(job.ScaleSetAssignTime),
		nullableTime(job.RunnerAssignTime),
		backendName,
	)
	if err != nil {
		slog.Error("failed to record job started", "job_id", job.JobID, "err", err)
		return
	}
	if diagnostics != nil {
		s.startCapture(job.JobID, job.RunnerName, diagnostics)
	}
}

func (s *JobStore) RecordJobMessageCompleted(job *scaleset.JobCompleted) {
	if _, ok := knownJobResults[job.Result]; !ok {
		slog.Error("unexpected job result from scale set API", "job_id", job.JobID, "result", job.Result)
		return
	}

	s.stopCapture(job.JobID)
	now := time.Now()
	if !job.FinishTime.IsZero() {
		now = job.FinishTime
	}
	res, err := s.queries.UpdateJobCompleted(context.Background(), sqlcdb.UpdateJobCompletedParams{
		Result:      job.Result,
		CompletedAt: sql.NullTime{Time: now, Valid: true},
		ID:          job.JobID,
	})
	if err != nil {
		slog.Error("failed to update job completed", "job_id", job.JobID, "err", err)
		return
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		slog.Error("failed to check completed job update", "job_id", job.JobID, "err", err)
		return
	}
	if rowsAffected == 0 {
		err = s.queries.InsertCompletedJob(context.Background(), sqlcdb.InsertCompletedJobParams{
			ID:          job.JobID,
			Result:      job.Result,
			StartedAt:   now,
			CompletedAt: sql.NullTime{Time: now, Valid: true},
		})
		if err != nil {
			slog.Error("failed to insert completed job", "job_id", job.JobID, "err", err)
		}
	}
}

func (s *JobStore) RecordJobStarted(setName, jobID, runnerName string) {
	s.RecordJobMessageStarted(setName, "", nil, &scaleset.JobStarted{
		RunnerName:     runnerName,
		JobMessageBase: scaleset.JobMessageBase{JobID: jobID},
	})
}

func (s *JobStore) RecordJobCompleted(jobID, result string) {
	s.RecordJobMessageCompleted(&scaleset.JobCompleted{
		Result:         result,
		JobMessageBase: scaleset.JobMessageBase{JobID: jobID},
	})
}

func (s *JobStore) startCapture(jobID, runnerName string, diagnostics backend.Diagnostics) {
	ctx, cancel := context.WithCancel(context.Background())
	state := &captureState{cancel: cancel, done: make(chan struct{})}
	s.captureMu.Lock()
	s.captures[jobID] = state
	s.captureMu.Unlock()

	go func() {
		defer close(state.done)
		logTicker := time.NewTicker(2 * time.Second)
		resourceTicker := time.NewTicker(5 * time.Second)
		defer logTicker.Stop()
		defer resourceTicker.Stop()
		s.captureResource(ctx, jobID, runnerName, diagnostics)
		for {
			select {
			case <-ctx.Done():
				finalCtx, finalCancel := context.WithTimeout(context.Background(), 5*time.Second)
				s.captureLogs(finalCtx, jobID, runnerName, diagnostics, state)
				s.captureResource(finalCtx, jobID, runnerName, diagnostics)
				finalCancel()
				return
			case <-logTicker.C:
				s.captureLogs(ctx, jobID, runnerName, diagnostics, state)
			case <-resourceTicker.C:
				s.captureResource(ctx, jobID, runnerName, diagnostics)
			}
		}
	}()
}

func (s *JobStore) stopCapture(jobID string) {
	s.captureMu.Lock()
	state := s.captures[jobID]
	delete(s.captures, jobID)
	s.captureMu.Unlock()
	if state != nil {
		state.cancel()
		select {
		case <-state.done:
		case <-time.After(6 * time.Second):
		}
	}
}

func (s *JobStore) captureLogs(ctx context.Context, jobID, runnerName string, diagnostics backend.Diagnostics, state *captureState) {
	text, err := diagnostics.ReadLogs(ctx, runnerName)
	if err != nil || len(text) <= state.logSize {
		return
	}
	added := text[state.logSize:]
	state.logSize = len(text)
	if len(added) > 1024*1024 {
		added = added[len(added)-1024*1024:]
	}
	if _, err := s.db.ExecContext(ctx, `INSERT INTO job_logs (job_id, recorded_at, text) VALUES (?, ?, ?)`, jobID, time.Now(), added); err != nil {
		slog.Warn("failed to record job logs", "job_id", jobID, "err", err)
	}
}

func (s *JobStore) captureResource(ctx context.Context, jobID, runnerName string, diagnostics backend.Diagnostics) {
	sample, err := diagnostics.ReadResource(ctx, runnerName)
	if err != nil {
		return
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO job_resource_samples (
			job_id, recorded_at, source, accuracy, cpu_percent, memory_used_bytes,
			memory_available_bytes, disk_used_bytes, disk_available_bytes,
			disk_read_bytes, disk_write_bytes, network_receive_bytes, network_send_bytes
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		jobID, sample.RecordedAt, sample.Source, sample.Accuracy, sample.CPUPercent,
		sample.MemoryUsedBytes, sample.MemoryAvailableBytes, sample.DiskUsedBytes,
		sample.DiskAvailableBytes, sample.DiskReadBytes, sample.DiskWriteBytes,
		sample.NetworkReceiveBytes, sample.NetworkSendBytes,
	)
	if err != nil {
		slog.Warn("failed to record job resource data", "job_id", jobID, "err", err)
	}
}

func (s *JobStore) Snapshot() []JobRecord {
	page := s.List(JobFilter{PageSize: 200})
	return page.Records
}

func (s *JobStore) List(filter JobFilter) JobPage {
	rows, err := s.db.QueryContext(context.Background(), `
		SELECT id, runner_name, runner_set_name, result, started_at, completed_at,
			owner, repository, workflow_ref, display_name, workflow_run_id, event_name,
			labels_json, queued_at, scale_set_assigned_at, runner_assigned_at, backend
		FROM jobs ORDER BY started_at DESC LIMIT 2000`)
	if err != nil {
		slog.Error("failed to list jobs", "err", err)
		return JobPage{}
	}
	defer rows.Close()

	var records []JobRecord
	for rows.Next() {
		record, scanErr := scanJob(rows)
		if scanErr != nil {
			slog.Error("failed to read job", "err", scanErr)
			continue
		}
		if matchesJob(record, filter) {
			records = append(records, record)
		}
	}
	start := max(filter.Cursor, 0)
	if start >= len(records) {
		return JobPage{}
	}
	pageSize := filter.PageSize
	if pageSize <= 0 || pageSize > 200 {
		pageSize = 50
	}
	end := min(start+pageSize, len(records))
	page := JobPage{Records: records[start:end]}
	if end < len(records) {
		page.NextCursor = strconv.Itoa(end)
	}
	return page
}

func (s *JobStore) Get(jobID string) (*JobRecord, error) {
	row := s.db.QueryRowContext(context.Background(), `
		SELECT id, runner_name, runner_set_name, result, started_at, completed_at,
			owner, repository, workflow_ref, display_name, workflow_run_id, event_name,
			labels_json, queued_at, scale_set_assigned_at, runner_assigned_at, backend
		FROM jobs WHERE id = ?`, jobID)
	record, err := scanJob(row)
	if err != nil {
		return nil, err
	}
	return &record, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanJob(row rowScanner) (JobRecord, error) {
	var record JobRecord
	var completedAt, queuedAt, scaleSetAssignedAt, runnerAssignedAt sql.NullTime
	var labels string
	err := row.Scan(
		&record.ID, &record.RunnerName, &record.RunnerSetName, &record.Result,
		&record.StartedAt, &completedAt, &record.Owner, &record.Repository,
		&record.WorkflowRef, &record.DisplayName, &record.WorkflowRunID,
		&record.EventName, &labels, &queuedAt, &scaleSetAssignedAt,
		&runnerAssignedAt, &record.Backend,
	)
	if err != nil {
		return JobRecord{}, err
	}
	_ = json.Unmarshal([]byte(labels), &record.Labels)
	record.CompletedAt = timePointer(completedAt)
	record.QueuedAt = timePointer(queuedAt)
	record.ScaleSetAssignedAt = timePointer(scaleSetAssignedAt)
	record.RunnerAssignedAt = timePointer(runnerAssignedAt)
	return record, nil
}

func matchesJob(record JobRecord, filter JobFilter) bool {
	if filter.Status != "" && !strings.EqualFold(record.Result, filter.Status) {
		return false
	}
	if filter.RunnerSet != "" && record.RunnerSetName != filter.RunnerSet {
		return false
	}
	repository := record.Owner + "/" + record.Repository
	if filter.Repository != "" && !strings.Contains(strings.ToLower(repository), strings.ToLower(filter.Repository)) {
		return false
	}
	if filter.Workflow != "" && !strings.Contains(strings.ToLower(record.WorkflowRef), strings.ToLower(filter.Workflow)) {
		return false
	}
	if filter.From != nil && record.StartedAt.Before(*filter.From) {
		return false
	}
	if filter.To != nil && record.StartedAt.After(*filter.To) {
		return false
	}
	return true
}

func (s *JobStore) Logs(jobID string, after int64, pageSize int) (logs []JobLog, nextSequence int64) {
	if pageSize <= 0 || pageSize > 500 {
		pageSize = 200
	}
	rows, err := s.db.QueryContext(context.Background(), `
		SELECT sequence, recorded_at, text FROM job_logs
		WHERE job_id = ? AND sequence > ? ORDER BY sequence LIMIT ?`,
		jobID, after, pageSize,
	)
	if err != nil {
		return nil, after
	}
	defer rows.Close()
	next := after
	for rows.Next() {
		var line JobLog
		if err := rows.Scan(&line.Sequence, &line.RecordedAt, &line.Text); err == nil {
			logs = append(logs, line)
			next = line.Sequence
		}
	}
	return logs, next
}

func (s *JobStore) Samples(jobID string) []ResourceSample {
	rows, err := s.db.QueryContext(context.Background(), `
		SELECT recorded_at, source, accuracy, cpu_percent, memory_used_bytes,
			memory_available_bytes, disk_used_bytes, disk_available_bytes,
			disk_read_bytes, disk_write_bytes, network_receive_bytes, network_send_bytes
		FROM job_resource_samples WHERE job_id = ? ORDER BY recorded_at`, jobID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var samples []ResourceSample
	for rows.Next() {
		var sample ResourceSample
		if err := rows.Scan(
			&sample.RecordedAt, &sample.Source, &sample.Accuracy, &sample.CPUPercent,
			&sample.MemoryUsedBytes, &sample.MemoryAvailableBytes, &sample.DiskUsedBytes,
			&sample.DiskAvailableBytes, &sample.DiskReadBytes, &sample.DiskWriteBytes,
			&sample.NetworkReceiveBytes, &sample.NetworkSendBytes,
		); err == nil {
			samples = append(samples, sample)
		}
	}
	return samples
}

func nullableTime(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return value
}

func timePointer(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	result := value.Time
	return &result
}
