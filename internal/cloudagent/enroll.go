package cloudagent

import (
	"context"
	"fmt"
	"os"
	"runtime"

	"connectrpc.com/connect"

	agentv1 "github.com/boring-design/elastic-fruit-protocol/gen/agent/v1"
	"github.com/boring-design/elastic-fruit-runner/internal/buildinfo"
	"github.com/boring-design/elastic-fruit-runner/internal/probe"
)

// EnrollInput is what the enroll command collects before calling the cloud.
type EnrollInput struct {
	ServerURL  string
	Token      string
	MaxRunners int
}

// EnrollResult is what the cloud handed back plus the probed backends.
type EnrollResult struct {
	Credential Credential
	AgentName  string
	Backends   []probe.BackendResult
}

// Enroll registers this host with the cloud using a one time token.
func Enroll(ctx context.Context, input EnrollInput) (EnrollResult, error) {
	client, err := newAgentClient(input.ServerURL, "")
	if err != nil {
		return EnrollResult{}, err
	}
	hostname, err := os.Hostname()
	if err != nil {
		return EnrollResult{}, fmt.Errorf("read hostname for enrollment: %w", err)
	}
	backends := ProbeBackends(ctx)
	response, err := client.Enroll(ctx, connect.NewRequest(&agentv1.EnrollRequest{
		EnrollmentToken:   input.Token,
		Hostname:          hostname,
		Os:                runtime.GOOS,
		Arch:              runtime.GOARCH,
		AgentVersion:      buildinfo.MainVersion(buildinfo.Current()),
		AvailableBackends: toProtoBackendCapabilities(backends),
		MaxRunners:        int32(input.MaxRunners),
		Isolation:         isolationFor(backends),
	}))
	if err != nil {
		return EnrollResult{}, fmt.Errorf("enroll with %s: %w", input.ServerURL, err)
	}
	if response.Msg.AgentId == "" || response.Msg.AgentCredential == "" {
		return EnrollResult{}, fmt.Errorf("enroll response from %s is missing agent_id or agent_credential", input.ServerURL)
	}
	return EnrollResult{
		Credential: Credential{
			AgentID:         response.Msg.AgentId,
			AgentCredential: response.Msg.AgentCredential,
		},
		AgentName: response.Msg.AgentName,
		Backends:  backends,
	}, nil
}
