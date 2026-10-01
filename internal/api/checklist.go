package api

import (
	"context"
	"runtime"
	"strings"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/boring-design/elastic-fruit-runner/config"
	controlplanev1 "github.com/boring-design/elastic-fruit-runner/gen/controlplane/v1"
	"github.com/boring-design/elastic-fruit-runner/internal/probe"
)

// githubProbe is the stored outcome of one GitHub target probe.
type githubProbe struct {
	target probe.GitHubTarget
	checks []probe.Check
}

// connectGracePeriod is how long after start a disconnected runner set is
// reported as still connecting instead of failed.
const connectGracePeriod = 2 * time.Minute

// RefreshProbes runs the GitHub and backend probes and stores the results.
// In config mode only docker is checked, plus tart on macOS.
func (s *Server) RefreshProbes(ctx context.Context) {
	var github []githubProbe
	var backends []probe.BackendResult
	if s.activeConfig == nil {
		for _, name := range configModeBackends() {
			backends = append(backends, probe.CheckBackend(ctx, name))
		}
	} else {
		for _, target := range configTargets(s.activeConfig) {
			github = append(github, githubProbe{
				target: target.GitHubTarget,
				checks: probe.TestGitHubAuth(ctx, target.GitHubTarget, target.auth),
			})
		}
		for _, name := range configBackends(s.activeConfig) {
			backends = append(backends, probe.CheckBackend(ctx, name))
		}
	}
	s.probeMu.Lock()
	s.githubProbes = github
	s.backendProbes = backends
	s.probedAt = time.Now()
	s.probeMu.Unlock()
}

func configModeBackends() []string {
	if runtime.GOOS == "darwin" {
		return []string{"docker", "tart"}
	}
	return []string{"docker"}
}

func (s *Server) GetSetupChecklist(ctx context.Context, req *connect.Request[controlplanev1.GetSetupChecklistRequest]) (*connect.Response[controlplanev1.GetSetupChecklistResponse], error) {
	if req.Msg.Refresh {
		s.RefreshProbes(ctx)
	}
	s.probeMu.Lock()
	github := s.githubProbes
	backends := s.backendProbes
	probedAt := s.probedAt
	s.probeMu.Unlock()

	steps := []*controlplanev1.SetupStep{s.configStep()}
	steps = append(steps, s.githubSteps(github)...)
	steps = append(steps, s.backendSteps(backends)...)
	steps = append(steps, s.runnerSetStep(), s.firstJobStep())

	response := &controlplanev1.GetSetupChecklistResponse{Steps: steps}
	if !probedAt.IsZero() {
		response.ProbedAt = timestamppb.New(probedAt)
	}
	return connect.NewResponse(response), nil
}

func (s *Server) configPath() string {
	if s.configState == nil {
		return ""
	}
	return s.configState.Get().Path
}

func (s *Server) configStep() *controlplanev1.SetupStep {
	if s.activeConfig == nil {
		return &controlplanev1.SetupStep{
			Id:      "config",
			Title:   "Config file",
			Status:  controlplanev1.StepStatus_STEP_STATUS_FAIL,
			Message: "No valid config file at " + s.configPath() + ". Use the setup wizard.",
			Page:    "setup",
		}
	}
	return &controlplanev1.SetupStep{
		Id:      "config",
		Title:   "Config file",
		Status:  controlplanev1.StepStatus_STEP_STATUS_PASS,
		Message: s.configPath(),
		Page:    "config",
	}
}

func (s *Server) githubSteps(results []githubProbe) []*controlplanev1.SetupStep {
	if s.activeConfig == nil {
		return []*controlplanev1.SetupStep{{
			Id:      "github_auth",
			Title:   "GitHub access",
			Status:  controlplanev1.StepStatus_STEP_STATUS_SKIPPED,
			Message: "Checked after the config is saved",
			Page:    "setup",
		}}
	}
	if len(results) == 0 {
		return []*controlplanev1.SetupStep{{
			Id:      "github_auth",
			Title:   "GitHub access",
			Status:  controlplanev1.StepStatus_STEP_STATUS_PENDING,
			Message: "checking",
			Page:    "config",
		}}
	}
	steps := make([]*controlplanev1.SetupStep, 0, len(results))
	for _, result := range results {
		step := &controlplanev1.SetupStep{
			Id:      "github_auth",
			Title:   "GitHub access: " + result.target.Name(),
			Status:  controlplanev1.StepStatus_STEP_STATUS_PASS,
			Message: "credentials and runner permission confirmed",
			Page:    "config",
		}
		if failure := probe.FirstFailure(result.checks); failure != "" {
			step.Status = controlplanev1.StepStatus_STEP_STATUS_FAIL
			step.Message = failure
		}
		steps = append(steps, step)
	}
	return steps
}

func (s *Server) backendSteps(results []probe.BackendResult) []*controlplanev1.SetupStep {
	page := "config"
	if s.activeConfig == nil {
		page = "setup"
	}
	if len(results) == 0 {
		return []*controlplanev1.SetupStep{{
			Id:      "backend",
			Title:   "Runner backend",
			Status:  controlplanev1.StepStatus_STEP_STATUS_PENDING,
			Message: "checking",
			Page:    page,
		}}
	}
	steps := make([]*controlplanev1.SetupStep, 0, len(results))
	for _, result := range results {
		step := &controlplanev1.SetupStep{
			Id:      "backend",
			Title:   "Runner backend: " + result.Backend,
			Status:  controlplanev1.StepStatus_STEP_STATUS_PASS,
			Message: result.Version,
			Page:    page,
		}
		if !result.Available {
			step.Status = controlplanev1.StepStatus_STEP_STATUS_FAIL
			step.Message = result.Error
		}
		steps = append(steps, step)
	}
	return steps
}

func (s *Server) runnerSetStep() *controlplanev1.SetupStep {
	step := &controlplanev1.SetupStep{
		Id:    "runner_sets",
		Title: "Runner sets connected",
		Page:  "runner-sets",
	}
	if s.activeConfig == nil || s.managementService == nil {
		step.Status = controlplanev1.StepStatus_STEP_STATUS_SKIPPED
		step.Message = "Checked after the config is saved"
		return step
	}
	uptime := time.Since(s.vitalsService.StartedAt())
	for _, runnerSet := range s.managementService.ListRunnerSets() {
		if runnerSet.Connected {
			continue
		}
		if uptime < connectGracePeriod {
			step.Status = controlplanev1.StepStatus_STEP_STATUS_PENDING
			step.Message = "connecting"
			return step
		}
		step.Status = controlplanev1.StepStatus_STEP_STATUS_FAIL
		step.Message = runnerSet.Info.Name + " is not connected, check the daemon log"
		return step
	}
	step.Status = controlplanev1.StepStatus_STEP_STATUS_PASS
	step.Message = "all runner sets are connected"
	return step
}

func (s *Server) firstJobStep() *controlplanev1.SetupStep {
	step := &controlplanev1.SetupStep{
		Id:    "first_job",
		Title: "First job",
		Page:  "jobs",
	}
	if s.activeConfig == nil || s.managementService == nil {
		step.Status = controlplanev1.StepStatus_STEP_STATUS_SKIPPED
		step.Message = "Checked after the config is saved"
		return step
	}
	jobs := s.managementService.ListJobRecords()
	if len(jobs) > 0 {
		job := jobs[0]
		name := job.DisplayName
		if name == "" {
			name = job.ID
		}
		step.Status = controlplanev1.StepStatus_STEP_STATUS_PASS
		step.Message = "latest job: " + name + " on " + job.StartedAt.Format("2006-01-02")
		return step
	}
	step.Status = controlplanev1.StepStatus_STEP_STATUS_PENDING
	step.Message = "push a workflow with runs-on: [" + firstRunnerSetLabels(s.activeConfig) + "]"
	return step
}

func firstRunnerSetLabels(cfg *config.Config) string {
	var labels []string
	switch {
	case len(cfg.Orgs) > 0 && len(cfg.Orgs[0].RunnerSets) > 0:
		labels = cfg.Orgs[0].RunnerSets[0].Labels
	case len(cfg.Repos) > 0 && len(cfg.Repos[0].RunnerSets) > 0:
		labels = cfg.Repos[0].RunnerSets[0].Labels
	}
	if len(labels) == 0 {
		return "self-hosted"
	}
	return strings.Join(labels, ", ")
}
