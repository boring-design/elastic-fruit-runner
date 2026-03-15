package scheduler

import "time"

// JobState tracks the lifecycle of a job through the scheduler.
type JobState int

const (
	JobPending   JobState = iota
	JobRunning
	JobCompleted
	JobFailed
)

func (s JobState) String() string {
	switch s {
	case JobPending:
		return "pending"
	case JobRunning:
		return "running"
	case JobCompleted:
		return "completed"
	case JobFailed:
		return "failed"
	default:
		return "unknown"
	}
}

// TrackedJob holds state for a job being processed by the scheduler.
type TrackedJob struct {
	JobID       string
	RunnerName  string
	State       JobState
	StartedAt   time.Time
	CompletedAt time.Time
}
