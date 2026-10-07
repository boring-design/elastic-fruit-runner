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
	sqlcdb "github.com/boring-design/elastic-fruit-runner/internal/storage/sqlc"
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
	cancel context.CancelFunc
	done   chan struct{}

	// mu guards jobID and logSize. The capture goroutine holds it while it
	// writes a row, and AttachJobMetadata holds it while it moves the job to
	// its real id, so no row is written under the old id after the move.
	mu      sync.Mutex
	jobID   string
	logSize int
}

type JobStore struct {
	db      *sql.DB
	queries *sqlcdb.Queries

	captureMu sync.Mutex
	captures  map[string]*captureState

	observerMu     sync.Mutex
	sampleObserver func(jobID string, sample backend.ResourceSample)
}

func NewJobStore(db *sql.DB) *JobStore {
	return &JobStore{
		db:       db,
		queries:  sqlcdb.New(db),
		captures: make(map[string]*captureState),
	}
}

// SetSampleObserver registers a function that is called with every resource
// sample after it is read, even when storing it failed. Passing nil turns the
// callback off.
func (s *JobStore) SetSampleObserver(fn func(jobID string, sample backend.ResourceSample)) {
	s.observerMu.Lock()
	defer s.observerMu.Unlock()
	s.sampleObserver = fn
}

func (s *JobStore) notifySampleObserver(jobID string, sample backend.ResourceSample) {
	s.observerMu.Lock()
	observer := s.sampleObserver
	s.observerMu.Unlock()
	if observer != nil {
		observer(jobID, sample)
	}
}

var knownJobResults = map[string]struct{}{
	"succeeded": {},
	"failed":    {},
	"canceled":  {},
}

func (s *JobStore) RecordJobMessageStarted(setName, backendName string, diagnostics backend.Diagnostics, job *scaleset.JobStarted) {
	err := s.queries.UpsertStartedJob(context.Background(), startedJobParams(setName, backendName, job))
	if err != nil {
		slog.Error("failed to record job started", "job_id", job.JobID, "err", err)
		return
	}
	if diagnostics != nil {
		s.startCapture(job.JobID, job.RunnerName, diagnostics)
	}
}

func startedJobParams(setName, backendName string, job *scaleset.JobStarted) sqlcdb.UpsertStartedJobParams {
	labels, _ := json.Marshal(job.RequestLabels)
	return sqlcdb.UpsertStartedJobParams{
		ID:                 job.JobID,
		RunnerName:         job.RunnerName,
		RunnerSetName:      setName,
		StartedAt:          time.Now(),
		Owner:              job.OwnerName,
		Repository:         job.RepositoryName,
		WorkflowRef:        job.JobWorkflowRef,
		DisplayName:        job.JobDisplayName,
		WorkflowRunID:      job.WorkflowRunID,
		EventName:          job.EventName,
		LabelsJson:         string(labels),
		QueuedAt:           nullableTime(job.QueueTime),
		ScaleSetAssignedAt: nullableTime(job.ScaleSetAssignTime),
		RunnerAssignedAt:   nullableTime(job.RunnerAssignTime),
		Backend:            backendName,
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
	state := &captureState{cancel: cancel, done: make(chan struct{}), jobID: jobID}
	s.captureMu.Lock()
	s.captures[jobID] = state
	s.captureMu.Unlock()

	go func() {
		defer close(state.done)
		logTicker := time.NewTicker(2 * time.Second)
		resourceTicker := time.NewTicker(5 * time.Second)
		defer logTicker.Stop()
		defer resourceTicker.Stop()
		s.captureResource(ctx, runnerName, diagnostics, state)
		for {
			select {
			case <-ctx.Done():
				finalCtx, finalCancel := context.WithTimeout(context.Background(), 5*time.Second)
				s.captureLogs(finalCtx, runnerName, diagnostics, state)
				s.captureResource(finalCtx, runnerName, diagnostics, state)
				finalCancel()
				return
			case <-logTicker.C:
				s.captureLogs(ctx, runnerName, diagnostics, state)
			case <-resourceTicker.C:
				s.captureResource(ctx, runnerName, diagnostics, state)
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

func (s *JobStore) captureLogs(ctx context.Context, runnerName string, diagnostics backend.Diagnostics, state *captureState) {
	text, err := diagnostics.ReadLogs(ctx, runnerName)
	if err != nil {
		return
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if len(text) <= state.logSize {
		return
	}
	added := text[state.logSize:]
	state.logSize = len(text)
	if len(added) > 1024*1024 {
		added = added[len(added)-1024*1024:]
	}
	err = s.queries.InsertJobLog(ctx, sqlcdb.InsertJobLogParams{
		JobID:      state.jobID,
		RecordedAt: time.Now(),
		Text:       added,
	})
	if err != nil {
		slog.Warn("failed to record job logs", "job_id", state.jobID, "runner", runnerName, "err", err)
	}
}

func (s *JobStore) captureResource(ctx context.Context, runnerName string, diagnostics backend.Diagnostics, state *captureState) {
	sample, err := diagnostics.ReadResource(ctx, runnerName)
	if err != nil {
		return
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	jobID := state.jobID
	err = s.queries.InsertJobResourceSample(ctx, sqlcdb.InsertJobResourceSampleParams{
		JobID:                jobID,
		RecordedAt:           sample.RecordedAt,
		Source:               sample.Source,
		Accuracy:             sample.Accuracy,
		CpuPercent:           sample.CPUPercent,
		MemoryUsedBytes:      sample.MemoryUsedBytes,
		MemoryAvailableBytes: sample.MemoryAvailableBytes,
		DiskUsedBytes:        sample.DiskUsedBytes,
		DiskAvailableBytes:   sample.DiskAvailableBytes,
		DiskReadBytes:        sample.DiskReadBytes,
		DiskWriteBytes:       sample.DiskWriteBytes,
		NetworkReceiveBytes:  sample.NetworkReceiveBytes,
		NetworkSendBytes:     sample.NetworkSendBytes,
	})
	if err != nil {
		slog.Warn("failed to record job resource data", "job_id", jobID, "err", err)
	}
	s.notifySampleObserver(jobID, sample)
}

func (s *JobStore) Snapshot() []JobRecord {
	page := s.List(JobFilter{PageSize: 200})
	return page.Records
}

func (s *JobStore) List(filter JobFilter) JobPage {
	rows, err := s.queries.ListJobsNewestFirst(context.Background())
	if err != nil {
		slog.Error("failed to list jobs", "err", err)
		return JobPage{}
	}

	var records []JobRecord
	for _, row := range rows {
		record := jobRecordFromRow(row)
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
	row, err := s.queries.GetJob(context.Background(), jobID)
	if err != nil {
		return nil, err
	}
	record := jobRecordFromRow(row)
	return &record, nil
}

func jobRecordFromRow(row sqlcdb.Job) JobRecord {
	record := JobRecord{
		ID:            row.ID,
		RunnerName:    row.RunnerName,
		RunnerSetName: row.RunnerSetName,
		Result:        row.Result,
		StartedAt:     row.StartedAt,
		Owner:         row.Owner,
		Repository:    row.Repository,
		WorkflowRef:   row.WorkflowRef,
		DisplayName:   row.DisplayName,
		WorkflowRunID: row.WorkflowRunID,
		EventName:     row.EventName,
		Backend:       row.Backend,
	}
	_ = json.Unmarshal([]byte(row.LabelsJson), &record.Labels)
	if row.CompletedAt.Valid {
		record.CompletedAt = &row.CompletedAt.Time
	}
	if row.QueuedAt.Valid {
		record.QueuedAt = &row.QueuedAt.Time
	}
	if row.ScaleSetAssignedAt.Valid {
		record.ScaleSetAssignedAt = &row.ScaleSetAssignedAt.Time
	}
	if row.RunnerAssignedAt.Valid {
		record.RunnerAssignedAt = &row.RunnerAssignedAt.Time
	}
	return record
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
	rows, err := s.queries.ListJobLogsAfterSequence(context.Background(), sqlcdb.ListJobLogsAfterSequenceParams{
		JobID:    jobID,
		Sequence: after,
		Limit:    int64(pageSize),
	})
	if err != nil {
		return nil, after
	}
	next := after
	for _, row := range rows {
		logs = append(logs, JobLog{Sequence: row.Sequence, RecordedAt: row.RecordedAt, Text: row.Text})
		next = row.Sequence
	}
	return logs, next
}

func (s *JobStore) Samples(jobID string) []ResourceSample {
	rows, err := s.queries.ListJobResourceSamples(context.Background(), jobID)
	if err != nil {
		return nil
	}
	var samples []ResourceSample
	for _, row := range rows {
		samples = append(samples, ResourceSample{
			RecordedAt:           row.RecordedAt,
			Source:               row.Source,
			Accuracy:             row.Accuracy,
			CPUPercent:           row.CpuPercent,
			MemoryUsedBytes:      row.MemoryUsedBytes,
			MemoryAvailableBytes: row.MemoryAvailableBytes,
			DiskUsedBytes:        row.DiskUsedBytes,
			DiskAvailableBytes:   row.DiskAvailableBytes,
			DiskReadBytes:        row.DiskReadBytes,
			DiskWriteBytes:       row.DiskWriteBytes,
			NetworkReceiveBytes:  row.NetworkReceiveBytes,
			NetworkSendBytes:     row.NetworkSendBytes,
		})
	}
	return samples
}

func nullableTime(value time.Time) sql.NullTime {
	if value.IsZero() {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: value, Valid: true}
}
