package api

import (
	"context"
	"errors"

	"connectrpc.com/connect"

	"github.com/boring-design/elastic-fruit-runner/config"
	controlplanev1 "github.com/boring-design/elastic-fruit-runner/gen/controlplane/v1"
	"github.com/boring-design/elastic-fruit-runner/internal/probe"
)

func (s *Server) TestGitHubAuth(ctx context.Context, req *connect.Request[controlplanev1.TestGitHubAuthRequest]) (*connect.Response[controlplanev1.TestGitHubAuthResponse], error) {
	msg := req.Msg
	if (msg.Org == "") == (msg.Repo == "") {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("set exactly one of org or repo"))
	}
	auth := config.AuthConfig{}
	switch {
	case msg.PatToken != "" && msg.GithubApp == nil:
		token := msg.PatToken
		auth.PATToken = &token
	case msg.PatToken == "" && msg.GithubApp != nil:
		auth.GitHubApp = &config.GitHubAppConfig{
			ClientID:       msg.GithubApp.ClientId,
			InstallationID: msg.GithubApp.InstallationId,
			PrivateKeyPath: msg.GithubApp.PrivateKeyPath,
		}
	default:
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("set exactly one of pat_token or github_app"))
	}
	target := probe.GitHubTarget{Org: msg.Org, Repo: msg.Repo, RunnerGroup: msg.RunnerGroup}
	checks := probe.TestGitHubAuth(ctx, target, &auth)
	return connect.NewResponse(toProtoGitHubAuth(target, checks)), nil
}

func (s *Server) CheckBackend(ctx context.Context, req *connect.Request[controlplanev1.CheckBackendRequest]) (*connect.Response[controlplanev1.CheckBackendResponse], error) {
	return connect.NewResponse(toProtoBackendCheck(probe.CheckBackend(ctx, req.Msg.Backend))), nil
}

func (s *Server) ProbeConfig(ctx context.Context, req *connect.Request[controlplanev1.ProbeConfigRequest]) (*connect.Response[controlplanev1.ProbeConfigResponse], error) {
	if s.configState == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("config service is unavailable"))
	}
	cfg, issues := s.configState.ParseWithSecrets([]byte(req.Msg.Yaml))
	response := &controlplanev1.ProbeConfigResponse{}
	if len(issues) > 0 {
		for _, issue := range issues {
			response.Errors = append(response.Errors, &controlplanev1.ConfigValidationIssue{Path: issue.Path, Message: issue.Message})
		}
		return connect.NewResponse(response), nil
	}
	for _, target := range configTargets(cfg) {
		checks := probe.TestGitHubAuth(ctx, target.GitHubTarget, target.auth)
		response.Targets = append(response.Targets, toProtoGitHubAuth(target.GitHubTarget, checks))
	}
	for _, name := range configBackends(cfg) {
		response.Backends = append(response.Backends, toProtoBackendCheck(probe.CheckBackend(ctx, name)))
	}
	return connect.NewResponse(response), nil
}

// configTarget pairs a GitHub target with the credentials from the config.
type configTarget struct {
	probe.GitHubTarget
	auth *config.AuthConfig
}

func configTargets(cfg *config.Config) []configTarget {
	var targets []configTarget
	for i := range cfg.Orgs {
		org := &cfg.Orgs[i]
		targets = append(targets, configTarget{
			GitHubTarget: probe.GitHubTarget{Org: org.Org, RunnerGroup: org.RunnerGroup},
			auth:         &org.Auth,
		})
	}
	for i := range cfg.Repos {
		repo := &cfg.Repos[i]
		targets = append(targets, configTarget{
			GitHubTarget: probe.GitHubTarget{Repo: repo.Repo},
			auth:         &repo.Auth,
		})
	}
	return targets
}

// configBackends returns each distinct backend name in config order.
func configBackends(cfg *config.Config) []string {
	seen := map[string]bool{}
	var names []string
	add := func(sets []config.RunnerSetConfig) {
		for _, set := range sets {
			if !seen[set.Backend] {
				seen[set.Backend] = true
				names = append(names, set.Backend)
			}
		}
	}
	for _, org := range cfg.Orgs {
		add(org.RunnerSets)
	}
	for _, repo := range cfg.Repos {
		add(repo.RunnerSets)
	}
	return names
}

func toProtoGitHubAuth(target probe.GitHubTarget, checks []probe.Check) *controlplanev1.TestGitHubAuthResponse {
	response := &controlplanev1.TestGitHubAuthResponse{Target: target.Name(), Ok: probe.Passed(checks)}
	for _, check := range checks {
		response.Checks = append(response.Checks, &controlplanev1.Check{
			Name:    check.Name,
			Status:  toProtoCheckStatus(check.Status),
			Message: check.Message,
		})
	}
	return response
}

func toProtoCheckStatus(status probe.Status) controlplanev1.CheckStatus {
	switch status {
	case probe.StatusPass:
		return controlplanev1.CheckStatus_CHECK_STATUS_PASS
	case probe.StatusFail:
		return controlplanev1.CheckStatus_CHECK_STATUS_FAIL
	case probe.StatusSkipped:
		return controlplanev1.CheckStatus_CHECK_STATUS_SKIPPED
	default:
		return controlplanev1.CheckStatus_CHECK_STATUS_UNSPECIFIED
	}
}

func toProtoBackendCheck(result probe.BackendResult) *controlplanev1.CheckBackendResponse {
	return &controlplanev1.CheckBackendResponse{
		Backend:   result.Backend,
		Available: result.Available,
		Version:   result.Version,
		HostOs:    result.HostOS,
		HostArch:  result.HostArch,
		Error:     result.Error,
	}
}
