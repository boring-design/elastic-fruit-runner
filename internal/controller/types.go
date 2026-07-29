package controller

import "time"

// RunnerState represents the lifecycle phase of a runner.
type RunnerState int

const (
	StatePreparing RunnerState = iota
	StateIdle
	StateBusy
)

// RunnerSetInfo holds the static configuration of a runner set.
type RunnerSetInfo struct {
	Name       string
	Backend    string
	Image      string
	Labels     []string
	MaxRunners int
}

// RunnerSnapshot is a point-in-time view of a single runner.
type RunnerSnapshot struct {
	Name  string
	State RunnerState
	Since time.Time
}

// JobRecorder records job lifecycle events.
// Implemented by the management service; injected into controllers.
//
// All four pieces of context (setName, jobID, runnerName, result) are passed
// through on completion so orphan completion records — written when the
// daemon never observed the corresponding start event, e.g. across restarts —
// still carry the runner name the dashboard needs to render them.
type JobRecorder interface {
	RecordJobStarted(setName, jobID, runnerName string)
	RecordJobCompleted(setName, jobID, runnerName, result string)
}
