package registry

import (
	"sync"
	"time"
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

// JobRing is a fixed-size ring buffer for job history.
// Thread-safe; uses its own mutex independent of Registry.
type JobRing struct {
	mu      sync.Mutex
	entries []JobRecord
	size    int
	cursor  int
	count   int
}

// NewJobRing creates a ring buffer with the given capacity.
func NewJobRing(size int) *JobRing {
	return &JobRing{
		entries: make([]JobRecord, size),
		size:    size,
	}
}

// RecordStarted inserts a new job with result "running".
func (r *JobRing) RecordStarted(jobID, runnerName, runnerSetName string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.entries[r.cursor] = JobRecord{
		ID:            jobID,
		RunnerName:    runnerName,
		RunnerSetName: runnerSetName,
		Result:        "running",
		StartedAt:     time.Now(),
	}
	r.cursor = (r.cursor + 1) % r.size
	if r.count < r.size {
		r.count++
	}
}

// RecordCompleted finds an existing job by ID and updates its result.
// If the job was evicted (ring wrapped), inserts a completed-only record.
func (r *JobRing) RecordCompleted(jobID, result string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	for i := range r.count {
		idx := (r.cursor - 1 - i + r.size) % r.size
		if r.entries[idx].ID == jobID {
			r.entries[idx].Result = result
			r.entries[idx].CompletedAt = &now
			return
		}
	}

	// Job was evicted from the ring (wrapped). Insert a completed-only record
	// so that completion events are not silently lost.
	// Set StartedAt to CompletedAt so dashboard duration math shows zero
	// instead of a multi-century value from a zero-time start.
	r.entries[r.cursor] = JobRecord{
		ID:          jobID,
		Result:      result,
		StartedAt:   now,
		CompletedAt: &now,
	}
	r.cursor = (r.cursor + 1) % r.size
	if r.count < r.size {
		r.count++
	}
}

// Snapshot returns a copy of all recorded jobs, most-recent-first.
func (r *JobRing) Snapshot() []JobRecord {
	r.mu.Lock()
	defer r.mu.Unlock()

	result := make([]JobRecord, r.count)
	for i := range r.count {
		idx := (r.cursor - 1 - i + r.size) % r.size
		result[i] = r.entries[idx]
	}
	return result
}
