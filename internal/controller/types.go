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

// JobStart carries the metadata captured when a job is first observed
// starting on a runner. The fields mirror scaleset.JobStarted /
// JobMessageBase. Empty strings are tolerated so the recorder can persist
// whatever the upstream event surfaces.
type JobStart struct {
	RunnerSetName string
	JobID         string
	RunnerName    string
	Repository    string
	WorkflowName  string
	WorkflowRunID string
}

// JobRecorder records job lifecycle events.
// Implemented by the management service; injected into controllers.
type JobRecorder interface {
	RecordJobStarted(start JobStart)
	RecordJobCompleted(jobID, result string)
}
