package management

import (
	"log/slog"
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

// JobStore is a fixed-size ring buffer for job history.
type JobStore struct {
	mu      sync.Mutex
	entries []JobRecord
	size    int
	cursor  int
	count   int
}

// NewJobStore creates a ring buffer with the given capacity.
func NewJobStore(size int) *JobStore {
	return &JobStore{
		entries: make([]JobRecord, size),
		size:    size,
	}
}

// RecordJobStarted inserts a new job with result "running".
func (s *JobStore) RecordJobStarted(setName, jobID, runnerName string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.entries[s.cursor] = JobRecord{
		ID:            jobID,
		RunnerName:    runnerName,
		RunnerSetName: setName,
		Result:        "running",
		StartedAt:     time.Now(),
	}
	s.cursor = (s.cursor + 1) % s.size
	if s.count < s.size {
		s.count++
	}
}

// knownJobResults is the set of valid result strings from the GitHub Actions API.
var knownJobResults = map[string]struct{}{
	"Succeeded": {},
	"Failed":    {},
}

// RecordJobCompleted finds an existing job by ID and updates its result.
// If the job was evicted (ring wrapped), inserts a completed-only record.
func (s *JobStore) RecordJobCompleted(jobID, result string) {
	if _, ok := knownJobResults[result]; !ok {
		slog.Warn("unexpected job result from scale-set API, recording as-is", "job_id", jobID, "result", result)
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for i := range s.count {
		idx := (s.cursor - 1 - i + s.size) % s.size
		if s.entries[idx].ID == jobID {
			s.entries[idx].Result = result
			s.entries[idx].CompletedAt = &now
			return
		}
	}

	// Job was evicted from the ring (wrapped). Insert a completed-only record
	// so that completion events are not silently lost.
	s.entries[s.cursor] = JobRecord{
		ID:          jobID,
		Result:      result,
		StartedAt:   now,
		CompletedAt: &now,
	}
	s.cursor = (s.cursor + 1) % s.size
	if s.count < s.size {
		s.count++
	}
}

// Snapshot returns a copy of all recorded jobs, most-recent-first.
func (s *JobStore) Snapshot() []JobRecord {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := make([]JobRecord, s.count)
	for i := range s.count {
		idx := (s.cursor - 1 - i + s.size) % s.size
		result[i] = s.entries[idx]
	}
	return result
}
