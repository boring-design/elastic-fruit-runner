package registry

import (
	"sync"
	"time"
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

// RunnerSetSnapshot is a point-in-time view of a runner set.
type RunnerSetSnapshot struct {
	Info      RunnerSetInfo
	Scope     string
	Connected bool
	Runners   []RunnerSnapshot
}

// MachineVitals holds host-level resource metrics.
type MachineVitals struct {
	CPUUsagePercent    float32
	MemoryUsagePercent float32
	DiskUsagePercent   float32
	TemperatureCelsius float32
}

type runner struct {
	state RunnerState
	since time.Time
}

type runnerSet struct {
	info      RunnerSetInfo
	scope     string
	connected bool
	runners   map[string]*runner
}

// Registry is a concurrency-safe central state store that aggregates
// state from all ScaleSetControllers for the API server to read.
type Registry struct {
	mu         sync.RWMutex
	startedAt  time.Time
	runnerSets map[string]*runnerSet
	jobs       *JobRing
	vitals     MachineVitals
}

// New creates a Registry with the given daemon start time.
func New(startedAt time.Time) *Registry {
	return &Registry{
		startedAt:  startedAt,
		runnerSets: make(map[string]*runnerSet),
		jobs:       NewJobRing(200),
	}
}

// RegisterRunnerSet registers a runner set's static config. Called once per set at startup.
func (r *Registry) RegisterRunnerSet(name string, info RunnerSetInfo, scope string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.runnerSets[name] = &runnerSet{
		info:    info,
		scope:   scope,
		runners: make(map[string]*runner),
	}
}

// SetConnected updates the GitHub connection status for a runner set.
func (r *Registry) SetConnected(setName string, connected bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if rs, ok := r.runnerSets[setName]; ok {
		rs.connected = connected
	}
}

// AddPreparing records a runner entering the preparing state.
func (r *Registry) AddPreparing(setName, runnerName string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if rs, ok := r.runnerSets[setName]; ok {
		rs.runners[runnerName] = &runner{state: StatePreparing, since: time.Now()}
	}
}

// MoveToIdle transitions a runner from preparing to idle.
func (r *Registry) MoveToIdle(setName, runnerName string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if rs, ok := r.runnerSets[setName]; ok {
		if rn, ok := rs.runners[runnerName]; ok {
			rn.state = StateIdle
			rn.since = time.Now()
		}
	}
}

// MarkBusy transitions a runner to busy.
func (r *Registry) MarkBusy(setName, runnerName string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if rs, ok := r.runnerSets[setName]; ok {
		if rn, ok := rs.runners[runnerName]; ok {
			rn.state = StateBusy
			rn.since = time.Now()
		}
	}
}

// MarkDone removes a runner from the registry.
func (r *Registry) MarkDone(setName, runnerName string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if rs, ok := r.runnerSets[setName]; ok {
		delete(rs.runners, runnerName)
	}
}

// ClearRunners removes all runners for a set. Called on controller shutdown.
func (r *Registry) ClearRunners(setName string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if rs, ok := r.runnerSets[setName]; ok {
		rs.runners = make(map[string]*runner)
	}
}

// RecordJobStarted inserts a new running job record.
func (r *Registry) RecordJobStarted(setName, jobID, runnerName string) {
	r.jobs.RecordStarted(jobID, runnerName, setName)
}

// RecordJobCompleted updates an existing job record with its result.
func (r *Registry) RecordJobCompleted(jobID, result string) {
	r.jobs.RecordCompleted(jobID, result)
}

// UpdateMachineVitals stores the latest host metrics snapshot.
func (r *Registry) UpdateMachineVitals(v MachineVitals) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.vitals = v
}

// --- Read side ---

// StartedAt returns when the daemon started.
func (r *Registry) StartedAt() time.Time {
	return r.startedAt
}

// Snapshot returns a point-in-time copy of all runner sets and their runners.
func (r *Registry) Snapshot() []RunnerSetSnapshot {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]RunnerSetSnapshot, 0, len(r.runnerSets))
	for _, rs := range r.runnerSets {
		runners := make([]RunnerSnapshot, 0, len(rs.runners))
		for name, rn := range rs.runners {
			runners = append(runners, RunnerSnapshot{
				Name:  name,
				State: rn.state,
				Since: rn.since,
			})
		}
		result = append(result, RunnerSetSnapshot{
			Info:      rs.info,
			Scope:     rs.scope,
			Connected: rs.connected,
			Runners:   runners,
		})
	}
	return result
}

// RecentJobs returns a copy of the job history, most-recent-first.
func (r *Registry) RecentJobs() []JobRecord {
	return r.jobs.Snapshot()
}

// GetMachineVitals returns the latest host metrics.
func (r *Registry) GetMachineVitals() MachineVitals {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.vitals
}
