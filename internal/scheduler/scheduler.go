package scheduler

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/actions/scaleset"

	"github.com/boring-design/elastic-fruit-runner/internal/runnerpool"
)

const (
	staleJobScanInterval = 5 * time.Minute
	staleJobMaxAge       = 30 * time.Minute
)

// Scheduler dispatches jobs to runner pool slots, deduplicating by JobID.
type Scheduler struct {
	pool       *runnerpool.RunnerPool
	client     *scaleset.Client
	scaleSetID int
	logger     *slog.Logger

	mu   sync.Mutex
	jobs map[string]*TrackedJob
}

// New creates a Scheduler.
func New(p *runnerpool.RunnerPool, client *scaleset.Client, scaleSetID int, logger *slog.Logger) *Scheduler {
	return &Scheduler{
		pool:       p,
		client:     client,
		scaleSetID: scaleSetID,
		logger:     logger,
		jobs:       make(map[string]*TrackedJob),
	}
}

// StartCleanup runs a background goroutine that removes stale terminal jobs.
func (s *Scheduler) StartCleanup(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(staleJobScanInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				s.cleanStaleJobs()
			case <-ctx.Done():
				return
			}
		}
	}()
}

// HandleJobAssigned deduplicates by JobID and launches a goroutine to run the job.
func (s *Scheduler) HandleJobAssigned(ctx context.Context, job *scaleset.JobAssigned) error {
	s.mu.Lock()
	if _, exists := s.jobs[job.JobID]; exists {
		s.mu.Unlock()
		s.logger.Info("duplicate job assignment ignored", "jobID", job.JobID)
		return nil
	}
	tracked := &TrackedJob{
		JobID:     job.JobID,
		State:     JobPending,
		StartedAt: time.Now(),
	}
	s.jobs[job.JobID] = tracked
	s.mu.Unlock()

	s.logger.Info("job assigned",
		"jobID", job.JobID,
		"repo", job.RepositoryName,
		"workflow", job.JobDisplayName,
	)

	go s.runJob(ctx, tracked)
	return nil
}

// HandleJobStarted logs the event and updates internal state.
func (s *Scheduler) HandleJobStarted(_ context.Context, job *scaleset.JobStarted) error {
	s.logger.Info("job started", "jobID", job.JobID, "runner", job.RunnerName, "runnerID", job.RunnerID)

	s.mu.Lock()
	if tracked, ok := s.jobs[job.JobID]; ok {
		tracked.State = JobRunning
	}
	s.mu.Unlock()
	return nil
}

// HandleJobCompleted logs the event and updates internal state.
func (s *Scheduler) HandleJobCompleted(_ context.Context, job *scaleset.JobCompleted) error {
	s.logger.Info("job completed", "jobID", job.JobID, "runner", job.RunnerName, "result", job.Result)

	s.mu.Lock()
	if tracked, ok := s.jobs[job.JobID]; ok {
		tracked.State = JobCompleted
		tracked.CompletedAt = time.Now()
	}
	s.mu.Unlock()
	return nil
}

// ActiveJobCount returns the number of non-terminal jobs.
func (s *Scheduler) ActiveJobCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	count := 0
	for _, j := range s.jobs {
		if j.State == JobPending || j.State == JobRunning {
			count++
		}
	}
	return count
}

func (s *Scheduler) runJob(ctx context.Context, tracked *TrackedJob) {
	log := s.logger.With("jobID", tracked.JobID)

	slot, err := s.pool.Acquire(ctx)
	if err != nil {
		log.Error("failed to acquire slot", "err", err)
		s.markFailed(tracked)
		return
	}

	s.mu.Lock()
	tracked.RunnerName = slot.Name
	s.mu.Unlock()
	log = log.With("runner", slot.Name, "slot", slot.ID)
	log.Info("slot acquired for job")

	defer s.pool.Release(slot)

	jitCfg, err := s.client.GenerateJitRunnerConfig(ctx,
		&scaleset.RunnerScaleSetJitRunnerSetting{Name: slot.Name},
		s.scaleSetID,
	)
	if err != nil {
		log.Error("generate JIT config failed", "err", err)
		s.markFailed(tracked)
		return
	}

	log.Info("running job on slot")
	if err := s.pool.Backend().RunRunner(ctx, slot.Name, jitCfg.EncodedJITConfig); err != nil {
		log.Error("runner failed", "err", err)
		// Try to remove the ghost runner from GitHub.
		if jitCfg.Runner != nil && jitCfg.Runner.ID > 0 {
			log.Info("removing ghost runner", "runnerID", jitCfg.Runner.ID)
			if rmErr := s.client.RemoveRunner(ctx, int64(jitCfg.Runner.ID)); rmErr != nil {
				log.Error("failed to remove ghost runner", "err", rmErr)
			}
		}
		s.markFailed(tracked)
		return
	}

	log.Info("job completed successfully")
}

func (s *Scheduler) markFailed(tracked *TrackedJob) {
	s.mu.Lock()
	tracked.State = JobFailed
	tracked.CompletedAt = time.Now()
	s.mu.Unlock()
}

func (s *Scheduler) cleanStaleJobs() {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := time.Now().Add(-staleJobMaxAge)
	removed := 0
	for id, j := range s.jobs {
		if (j.State == JobCompleted || j.State == JobFailed) && j.CompletedAt.Before(cutoff) {
			delete(s.jobs, id)
			removed++
		}
	}
	if removed > 0 {
		s.logger.Info("cleaned stale jobs", "removed", removed)
	}
}
