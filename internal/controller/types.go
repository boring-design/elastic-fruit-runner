package controller

import (
	"time"

	"github.com/actions/scaleset"

	"github.com/boring-design/elastic-fruit-runner/internal/backend"
)

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
type JobRecorder interface {
	RecordJobMessageStarted(setName, backendName string, diagnostics backend.Diagnostics, job *scaleset.JobStarted)
	RecordJobMessageCompleted(job *scaleset.JobCompleted)
}
