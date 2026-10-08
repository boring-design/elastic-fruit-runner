package cloudagent

import (
	"context"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	agentv1 "github.com/boring-design/elastic-fruit-protocol/gen/agent/v1"
)

const heartbeatInterval = 10 * time.Second

// runHeartbeatLoop sends host vitals and runner states every ten seconds.
func (s *Service) runHeartbeatLoop(ctx context.Context) {
	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()
	s.sendHeartbeat(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.sendHeartbeat(ctx)
		}
	}
}

func (s *Service) sendHeartbeat(ctx context.Context) {
	callCtx, cancel := context.WithTimeout(ctx, heartbeatInterval)
	defer cancel()
	_, err := s.client.Heartbeat(callCtx, connect.NewRequest(s.heartbeatRequest()))
	if err == nil || ctx.Err() != nil {
		return
	}
	// When the command stream is down the reconnect loop already warned once,
	// so repeated heartbeat failures stay at debug level.
	if s.streamUp.Load() {
		s.logger.Warn("heartbeat failed", "server_url", s.serverURL, "err", err)
	} else {
		s.logger.Debug("heartbeat failed while the command stream is down", "server_url", s.serverURL, "err", err)
	}
}

func (s *Service) heartbeatRequest() *agentv1.HeartbeatRequest {
	snapshot := s.tracker.Snapshot()
	runners := make([]*agentv1.RunnerStatus, 0, len(snapshot))
	for _, runner := range snapshot {
		runners = append(runners, &agentv1.RunnerStatus{
			Name:  runner.Name,
			State: toProtoRunnerState(runner.State),
			Since: timestamppb.New(runner.Since),
		})
	}
	return &agentv1.HeartbeatRequest{
		HostVitals: toProtoHostVitals(s.vitals.GetVitals()),
		Runners:    runners,
		MaxRunners: int32(s.maxRunners),
		Isolation:  isolationFor(s.availableBackends),
	}
}
