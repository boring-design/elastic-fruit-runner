// Package cloudagent runs the daemon in cloud mode. An Elastic Fruit Cloud
// server talks to GitHub and pushes runner commands to this agent over a
// server stream. Jobs, logs, and samples are still recorded in local SQLite
// so the local console keeps working.
package cloudagent

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/boring-design/elastic-fruit-runner/config"
	"github.com/boring-design/elastic-fruit-runner/gen/agent/v1/agentv1connect"
	"github.com/boring-design/elastic-fruit-runner/internal/backend"
	"github.com/boring-design/elastic-fruit-runner/internal/controller"
	"github.com/boring-design/elastic-fruit-runner/internal/management"
	"github.com/boring-design/elastic-fruit-runner/internal/probe"
	"github.com/boring-design/elastic-fruit-runner/internal/vitals"
)

// Service is the cloud mode counterpart of management.Service.
type Service struct {
	serverURL  string
	maxRunners int
	client     agentv1connect.AgentServiceClient
	jobs       *management.JobStore
	host       *management.HostStore
	vitals     *vitals.Service
	tracker    *controller.RunnerTracker
	logger     *slog.Logger

	// availableBackends is the probe result sent at enrollment time and kept for the console.
	availableBackends []probe.BackendResult

	// streamUp is true while the command stream delivers messages.
	streamUp atomic.Bool

	mu         sync.Mutex
	backends   map[backendKey]backend.Backend
	runnerSets map[string]*runnerSetState
	runners    map[string]runnerInfo
	jobRunners map[string]string

	samples chan sampleReport
	wg      sync.WaitGroup
}

// backendKey identifies one backend instance. Two runner sets with the same
// backend, image, and platform share one instance.
type backendKey struct {
	backend  string
	image    string
	platform string
}

// runnerSetState is what the agent knows about a runner set it has seen.
type runnerSetState struct {
	backend  string
	image    string
	platform string
	// cleanupOnce removes leftovers from previous runs the first time the set is used.
	cleanupOnce sync.Once
}

// runnerInfo links a runner to its set and to the backend that runs it.
type runnerInfo struct {
	setName string
	backend backend.Backend
}

// New builds the cloud agent. It probes the local backends once so the
// result can be shown in the console, but it does not talk to the cloud yet.
func New(ctx context.Context, cfg *config.Config, credential Credential, db *sql.DB, vitalsService *vitals.Service) (*Service, error) {
	if cfg.Cloud == nil {
		return nil, fmt.Errorf("cloud agent needs a cloud block in config %s", cfg.FilePath)
	}
	databasePath, err := cfg.DatabasePath()
	if err != nil {
		return nil, fmt.Errorf("resolve database path: %w", err)
	}
	client, err := newAgentClient(cfg.Cloud.ServerURL, credential.AgentCredential)
	if err != nil {
		return nil, err
	}
	svc := &Service{
		serverURL:         cfg.Cloud.ServerURL,
		maxRunners:        cfg.Cloud.MaxRunners,
		client:            client,
		jobs:              management.NewJobStore(db),
		host:              management.NewHostStore(db, databasePath),
		vitals:            vitalsService,
		tracker:           controller.NewRunnerTracker(),
		logger:            slog.Default().With("component", "cloudagent"),
		availableBackends: ProbeBackends(ctx),
		backends:          make(map[backendKey]backend.Backend),
		runnerSets:        make(map[string]*runnerSetState),
		runners:           make(map[string]runnerInfo),
		jobRunners:        make(map[string]string),
		samples:           make(chan sampleReport, sampleQueueSize),
	}
	svc.jobs.SetSampleObserver(svc.observeSample)
	return svc, nil
}

// Start runs the command, heartbeat, and sample upload loops until ctx ends.
// Runners that are already running are left alone when ctx ends.
func (s *Service) Start(ctx context.Context) {
	s.wg.Add(3)
	go func() {
		defer s.wg.Done()
		s.runCommandLoop(ctx)
	}()
	go func() {
		defer s.wg.Done()
		s.runHeartbeatLoop(ctx)
	}()
	go func() {
		defer s.wg.Done()
		s.runSampleSender(ctx)
	}()
}

// Wait blocks until every loop started by Start has stopped.
func (s *Service) Wait() {
	s.wg.Wait()
}

// Connected reports whether the command stream to the cloud is up.
func (s *Service) Connected() bool {
	return s.streamUp.Load()
}

// AvailableBackends returns the backends that passed the startup probe.
func (s *Service) AvailableBackends() []probe.BackendResult {
	return append([]probe.BackendResult(nil), s.availableBackends...)
}

// RecordHostVitals stores one host resource sample in local SQLite.
func (s *Service) RecordHostVitals(value vitals.Vitals) {
	s.host.RecordHostVitals(value)
}

// ListRunnerSets returns one view per runner set seen since the daemon started.
func (s *Service) ListRunnerSets() []management.RunnerSetView {
	connected := s.streamUp.Load()
	runners := s.tracker.Snapshot()

	s.mu.Lock()
	names := make([]string, 0, len(s.runnerSets))
	for name := range s.runnerSets {
		names = append(names, name)
	}
	sort.Strings(names)
	runnerSet := make(map[string]string, len(s.runners))
	for name, info := range s.runners {
		runnerSet[name] = info.setName
	}
	views := make([]management.RunnerSetView, 0, len(names))
	for _, name := range names {
		state := s.runnerSets[name]
		view := management.RunnerSetView{
			Info: controller.RunnerSetInfo{
				Name:       name,
				Backend:    state.backend,
				Image:      state.image,
				MaxRunners: s.maxRunners,
			},
			Scope:     "cloud: " + s.serverURL,
			Connected: connected,
			Runners:   []controller.RunnerSnapshot{},
		}
		for _, runner := range runners {
			if runnerSet[runner.Name] == name {
				view.Runners = append(view.Runners, runner)
			}
		}
		views = append(views, view)
	}
	s.mu.Unlock()
	return views
}

// ListJobRecords returns job history, most recent first.
func (s *Service) ListJobRecords() []management.JobRecord {
	return s.jobs.Snapshot()
}

func (s *Service) FindJobRecords(filter management.JobFilter) management.JobPage {
	return s.jobs.List(filter)
}

func (s *Service) GetJobRecord(jobID string) (*management.JobRecord, error) {
	return s.jobs.Get(jobID)
}

func (s *Service) GetJobLogs(jobID string, after int64, pageSize int) (logs []management.JobLog, nextSequence int64) {
	return s.jobs.Logs(jobID, after, pageSize)
}

func (s *Service) GetJobSamples(jobID string) []management.ResourceSample {
	return s.jobs.Samples(jobID)
}

func (s *Service) HostSamples(from, to time.Time) ([]management.HostSample, *time.Time) {
	return s.host.HostSamples(from, to)
}

// backendFor returns the shared backend instance for the given settings.
func (s *Service) backendFor(name, image, platform string) (backend.Backend, error) {
	key := backendKey{backend: name, image: image, platform: platform}
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, ok := s.backends[key]; ok {
		return existing, nil
	}
	created, err := backend.New(name, image, platform)
	if err != nil {
		return nil, err
	}
	s.backends[key] = created
	return created, nil
}

// rememberRunnerSet records a runner set the first time a command mentions it.
func (s *Service) rememberRunnerSet(name, backendName, image, platform string) *runnerSetState {
	s.mu.Lock()
	defer s.mu.Unlock()
	state, ok := s.runnerSets[name]
	if !ok {
		state = &runnerSetState{}
		s.runnerSets[name] = state
	}
	state.backend = backendName
	if image != "" {
		state.image = image
	}
	if platform != "" {
		state.platform = platform
	}
	return state
}

// trackRunner starts tracking a runner. It returns false when the runner is
// already known, which happens when the cloud sends a command again.
func (s *Service) trackRunner(name, setName string, b backend.Backend) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, known := s.runners[name]; known {
		return false
	}
	s.runners[name] = runnerInfo{setName: setName, backend: b}
	s.tracker.MarkStarting(name)
	return true
}

// forgetRunner stops tracking one runner and returns what was known about it.
func (s *Service) forgetRunner(name string) (runnerInfo, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	info, known := s.runners[name]
	delete(s.runners, name)
	s.tracker.Remove(name)
	return info, known
}

// forgetRunnerSet stops tracking every runner of a set and returns their
// names plus the backends that may hold their resources.
func (s *Service) forgetRunnerSet(setName string) (removed []string, backends []backend.Backend) {
	s.mu.Lock()
	defer s.mu.Unlock()
	seen := make(map[backend.Backend]struct{})
	for name, info := range s.runners {
		if info.setName != setName {
			continue
		}
		removed = append(removed, name)
		delete(s.runners, name)
		s.tracker.Remove(name)
		if info.backend != nil {
			if _, ok := seen[info.backend]; !ok {
				seen[info.backend] = struct{}{}
				backends = append(backends, info.backend)
			}
		}
	}
	if state, ok := s.runnerSets[setName]; ok {
		key := backendKey{backend: state.backend, image: state.image, platform: state.platform}
		if b, ok := s.backends[key]; ok {
			if _, dup := seen[b]; !dup {
				backends = append(backends, b)
			}
		}
	}
	sort.Strings(removed)
	return removed, backends
}

// runnerState returns the tracked phase of one runner.
func (s *Service) runnerState(name string) (controller.RunnerState, bool) {
	for _, runner := range s.tracker.Snapshot() {
		if runner.Name == name {
			return runner.State, true
		}
	}
	return 0, false
}

// diagnosticsFor finds the backend that can read logs and resource data for
// a runner. It falls back to any backend with the same name.
func (s *Service) diagnosticsFor(runnerName, backendName string) backend.Diagnostics {
	s.mu.Lock()
	defer s.mu.Unlock()
	if info, ok := s.runners[runnerName]; ok && info.backend != nil {
		diagnostics, _ := info.backend.(backend.Diagnostics)
		return diagnostics
	}
	for key, b := range s.backends {
		if key.backend == backendName {
			diagnostics, _ := b.(backend.Diagnostics)
			return diagnostics
		}
	}
	return nil
}

// ProbeBackends checks docker and tart and returns only the working ones.
func ProbeBackends(ctx context.Context) []probe.BackendResult {
	var available []probe.BackendResult
	for _, name := range []string{"docker", "tart"} {
		result := probe.CheckBackend(ctx, name)
		if result.Available {
			available = append(available, result)
		}
	}
	return available
}
