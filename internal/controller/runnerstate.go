package controller

import (
	"context"
	"sync"
	"time"
)

// RunnerTracker tracks the lifecycle phase of each runner.
// preparing: VM is being cloned/started, no job assigned yet.
// idle:       Runner process is up, waiting for GitHub to assign a job.
//
//	The value is the time the runner entered idle state, used for
//	idle timeout eviction (keepAliveTime semantics).
//
// busy:       Runner has picked up a job and is executing it.
type RunnerTracker struct {
	mu        sync.Mutex
	runnerCtx context.Context
	preparing map[string]time.Time
	idle      map[string]time.Time
	busy      map[string]time.Time
}

// NewRunnerTracker returns an empty tracker.
func NewRunnerTracker() *RunnerTracker {
	return &RunnerTracker{
		preparing: make(map[string]time.Time),
		idle:      make(map[string]time.Time),
		busy:      make(map[string]time.Time),
	}
}

// RunnerCounts holds the number of runners in each phase.
type RunnerCounts struct {
	Preparing int
	Idle      int
	Busy      int
}

// Total returns the number of runners across all phases.
func (c RunnerCounts) Total() int {
	return c.Preparing + c.Idle + c.Busy
}

func (r *RunnerTracker) setRunnerCtx(ctx context.Context) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.runnerCtx = ctx
}

func (r *RunnerTracker) getRunnerCtx() context.Context {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.runnerCtx
}

// Count returns the number of runners across all phases.
func (r *RunnerTracker) Count() int {
	return r.Counts().Total()
}

// Counts returns the number of runners in each phase.
func (r *RunnerTracker) Counts() RunnerCounts {
	r.mu.Lock()
	defer r.mu.Unlock()
	return RunnerCounts{
		Preparing: len(r.preparing),
		Idle:      len(r.idle),
		Busy:      len(r.busy),
	}
}

// MarkStarting records a runner that is being prepared.
func (r *RunnerTracker) MarkStarting(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.preparing == nil {
		r.preparing = make(map[string]time.Time)
	}
	r.preparing[name] = time.Now()
}

// MarkIdle moves a runner from preparing to idle.
func (r *RunnerTracker) MarkIdle(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.preparing, name)
	if r.idle == nil {
		r.idle = make(map[string]time.Time)
	}
	r.idle[name] = time.Now()
}

// MarkBusy moves a runner from idle to busy.
func (r *RunnerTracker) MarkBusy(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.idle, name)
	if r.busy == nil {
		r.busy = make(map[string]time.Time)
	}
	r.busy[name] = time.Now()
}

// Remove drops the runner from whichever phase it is in.
// Safe to call multiple times (idempotent).
func (r *RunnerTracker) Remove(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.preparing, name)
	delete(r.idle, name)
	delete(r.busy, name)
}

// removeAll forgets every runner and returns the names of idle and busy
// runners that need cleanup, plus how many preparing runners were dropped.
func (r *RunnerTracker) removeAll() (toCleanup []string, preparingCount int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	toCleanup = make([]string, 0, len(r.idle)+len(r.busy))
	for name := range r.idle {
		toCleanup = append(toCleanup, name)
	}
	for name := range r.busy {
		toCleanup = append(toCleanup, name)
	}
	preparingCount = len(r.preparing)
	r.preparing = make(map[string]time.Time)
	r.idle = make(map[string]time.Time)
	r.busy = make(map[string]time.Time)
	return toCleanup, preparingCount
}

// removeIdleOlderThan drops idle runners that have been idle longer than
// timeout as of now and returns their names.
func (r *RunnerTracker) removeIdleOlderThan(now time.Time, timeout time.Duration) []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	var expired []string
	for name, idleSince := range r.idle {
		if now.Sub(idleSince) > timeout {
			expired = append(expired, name)
		}
	}
	for _, name := range expired {
		delete(r.idle, name)
	}
	return expired
}

// Snapshot returns a point-in-time copy of all runners.
func (r *RunnerTracker) Snapshot() []RunnerSnapshot {
	r.mu.Lock()
	defer r.mu.Unlock()

	result := make([]RunnerSnapshot, 0, len(r.preparing)+len(r.idle)+len(r.busy))
	for name, since := range r.preparing {
		result = append(result, RunnerSnapshot{Name: name, State: StatePreparing, Since: since})
	}
	for name, since := range r.idle {
		result = append(result, RunnerSnapshot{Name: name, State: StateIdle, Since: since})
	}
	for name, since := range r.busy {
		result = append(result, RunnerSnapshot{Name: name, State: StateBusy, Since: since})
	}
	return result
}
