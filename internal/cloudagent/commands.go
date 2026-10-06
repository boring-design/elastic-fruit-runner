package cloudagent

import (
	"context"
	"errors"
	"math/rand/v2"
	"time"

	"connectrpc.com/connect"

	agentv1 "github.com/boring-design/elastic-fruit-runner/gen/agent/v1"
	"github.com/boring-design/elastic-fruit-runner/internal/controller"
)

const (
	initialReconnectDelay = time.Second
	maxReconnectDelay     = 30 * time.Second
	// streamIdleTimeout is how long the stream may stay silent before the agent
	// treats the connection as dead and reconnects. The cloud sends Keepalive
	// messages well inside this window.
	streamIdleTimeout = 90 * time.Second
	reportTimeout     = 15 * time.Second
)

// runCommandLoop keeps the command stream open and reconnects with backoff.
// It logs once when the stream goes down and once when it is back.
func (s *Service) runCommandLoop(ctx context.Context) {
	delay := initialReconnectDelay
	downLogged := false
	for {
		wasUp, err := s.watchCommands(ctx)
		if ctx.Err() != nil {
			s.logger.Info("command stream stopped", "server_url", s.serverURL)
			return
		}
		if wasUp {
			delay = initialReconnectDelay
			downLogged = false
		}
		if downLogged {
			s.logger.Debug("command stream still down", "server_url", s.serverURL, "err", err, "retry_in", delay.String())
		} else {
			s.logger.Warn("command stream down, reconnecting with backoff", "server_url", s.serverURL, "err", err, "retry_in", delay.String())
			downLogged = true
		}
		if !sleepWithContext(ctx, withJitter(delay)) {
			return
		}
		delay = nextReconnectDelay(delay)
	}
}

// watchCommands opens one stream and handles commands until it breaks.
// wasUp tells whether at least one message arrived on this stream.
func (s *Service) watchCommands(ctx context.Context) (wasUp bool, err error) {
	streamCtx, cancelStream := context.WithCancel(ctx)
	defer cancelStream()
	idleTimer := time.AfterFunc(streamIdleTimeout, cancelStream)
	defer idleTimer.Stop()

	stream, err := s.client.WatchCommands(streamCtx, connect.NewRequest(&agentv1.WatchCommandsRequest{}))
	if err != nil {
		return false, err
	}
	defer stream.Close()

	for stream.Receive() {
		idleTimer.Reset(streamIdleTimeout)
		if !wasUp {
			wasUp = true
			s.streamUp.Store(true)
			s.logger.Info("command stream connected", "server_url", s.serverURL)
		}
		s.handleCommand(ctx, stream.Msg())
	}
	s.streamUp.Store(false)

	err = stream.Err()
	switch {
	case ctx.Err() != nil:
		return wasUp, ctx.Err()
	case streamCtx.Err() != nil:
		return wasUp, errors.New("no message from the cloud for " + streamIdleTimeout.String())
	case err == nil:
		return wasUp, errors.New("cloud closed the command stream")
	default:
		return wasUp, err
	}
}

// handleCommand dispatches one command. Runner commands run in their own
// goroutine because backends take seconds to minutes. Job bookkeeping runs
// inline so a JobFinished never overtakes its JobAssigned.
func (s *Service) handleCommand(ctx context.Context, command *agentv1.AgentCommand) {
	switch payload := command.Command.(type) {
	case *agentv1.AgentCommand_StartRunner:
		go s.handleStartRunner(ctx, command.CommandId, payload.StartRunner)
	case *agentv1.AgentCommand_CleanupRunner:
		go s.handleCleanupRunner(ctx, command.CommandId, payload.CleanupRunner.RunnerName)
	case *agentv1.AgentCommand_CleanupRunnerSet:
		go s.handleCleanupRunnerSet(ctx, command.CommandId, payload.CleanupRunnerSet.RunnerSetName)
	case *agentv1.AgentCommand_JobAssigned:
		s.handleJobAssigned(payload.JobAssigned)
	case *agentv1.AgentCommand_JobFinished:
		s.handleJobFinished(payload.JobFinished)
	case *agentv1.AgentCommand_Keepalive:
	default:
		s.logger.Warn("unknown command ignored", "command_id", command.CommandId)
	}
}

// handleStartRunner starts one runner and reports the outcome. A command the
// cloud sends again for a runner that is already up is answered again
// without starting anything.
func (s *Service) handleStartRunner(ctx context.Context, commandID string, command *agentv1.StartRunner) {
	// Backend work must survive daemon shutdown, so it does not inherit cancellation.
	ctx = context.WithoutCancel(ctx)
	log := s.logger.With(
		"command_id", commandID,
		"runner", command.RunnerName,
		"runner_set", command.RunnerSetName,
		"backend", command.Backend,
	)
	b, err := s.backendFor(command.Backend, command.Image, command.Platform)
	if err != nil {
		log.Error("start runner rejected", "err", err)
		s.reportEvent(ctx, runnerStartFailedEvent(commandID, command.RunnerName, err))
		return
	}
	setState := s.rememberRunnerSet(command.RunnerSetName, command.Backend, command.Image, command.Platform)
	setState.cleanupOnce.Do(func() {
		log.Info("cleaning up runners from previous runs")
		b.CleanupAll(ctx, command.RunnerSetName)
	})

	if !s.trackRunner(command.RunnerName, command.RunnerSetName, b) {
		state, _ := s.runnerState(command.RunnerName)
		if state == controller.StatePreparing {
			log.Info("runner is already starting, waiting for the first attempt")
			return
		}
		log.Info("runner already running, acknowledging again")
		s.reportEvent(ctx, runnerStartedEvent(commandID, command.RunnerName))
		return
	}

	log.Info("preparing runner")
	if err := b.Run(ctx, command.RunnerName, command.JitConfig); err != nil {
		log.Error("start runner failed", "err", err)
		s.forgetRunner(command.RunnerName)
		b.Cleanup(ctx, command.RunnerName)
		s.reportEvent(ctx, runnerStartFailedEvent(commandID, command.RunnerName, err))
		return
	}
	s.tracker.MarkIdle(command.RunnerName)
	log.Info("runner started, waiting for job assignment")
	s.reportEvent(ctx, runnerStartedEvent(commandID, command.RunnerName))
}

// handleCleanupRunner removes one runner. An unknown runner is reported as
// cleaned too so the cloud can close the command.
func (s *Service) handleCleanupRunner(ctx context.Context, commandID, runnerName string) {
	ctx = context.WithoutCancel(ctx)
	log := s.logger.With("command_id", commandID, "runner", runnerName)
	info, known := s.forgetRunner(runnerName)
	if known && info.backend != nil {
		log.Info("cleaning up runner")
		info.backend.Cleanup(ctx, runnerName)
	} else {
		log.Info("cleanup for unknown runner, nothing to remove")
	}
	s.reportEvent(ctx, runnerCleanedEvent(commandID, runnerName))
}

// handleCleanupRunnerSet removes every runner of a set and reports one
// RunnerCleaned per removed runner, each echoing the set command id.
func (s *Service) handleCleanupRunnerSet(ctx context.Context, commandID, setName string) {
	ctx = context.WithoutCancel(ctx)
	log := s.logger.With("command_id", commandID, "runner_set", setName)
	removed, backends := s.forgetRunnerSet(setName)
	log.Info("cleaning up runner set", "runners", removed)
	for _, b := range backends {
		b.CleanupAll(ctx, setName)
	}
	for _, runnerName := range removed {
		s.reportEvent(ctx, runnerCleanedEvent(commandID, runnerName))
	}
}

// handleJobAssigned records the job locally and starts log and sample capture.
func (s *Service) handleJobAssigned(command *agentv1.JobAssigned) {
	if command.Job == nil {
		s.logger.Warn("job assigned without job metadata ignored", "runner", command.RunnerName)
		return
	}
	s.rememberRunnerSet(command.RunnerSetName, command.Backend, "", "")
	s.tracker.MarkBusy(command.RunnerName)
	diagnostics := s.diagnosticsFor(command.RunnerName, command.Backend)
	s.rememberJobRunner(command.Job.JobId, command.RunnerName)
	s.jobs.RecordJobMessageStarted(command.RunnerSetName, command.Backend, diagnostics, toJobStarted(command))
	s.logger.Info("job started", "runner", command.RunnerName, "job_id", command.Job.JobId, "runner_set", command.RunnerSetName)
}

// handleJobFinished closes the local job record and stops capture.
func (s *Service) handleJobFinished(command *agentv1.JobFinished) {
	result, known := jobResultString(command.Result)
	if !known {
		s.logger.Warn("job finished with unknown result, recording it as failed", "job_id", command.JobId, "result", command.Result.String())
		result = "failed"
	}
	s.tracker.MarkIdle(command.RunnerName)
	s.jobs.RecordJobMessageCompleted(toJobCompleted(command, result))
	s.forgetJobRunner(command.JobId)
	s.logger.Info("job completed", "runner", command.RunnerName, "job_id", command.JobId, "result", result)
}

func runnerStartedEvent(commandID, runnerName string) *agentv1.ReportRunnerEventRequest {
	return &agentv1.ReportRunnerEventRequest{
		CommandId:  commandID,
		RunnerName: runnerName,
		Event:      &agentv1.ReportRunnerEventRequest_Started{Started: &agentv1.RunnerStarted{}},
	}
}

func runnerStartFailedEvent(commandID, runnerName string, cause error) *agentv1.ReportRunnerEventRequest {
	return &agentv1.ReportRunnerEventRequest{
		CommandId:  commandID,
		RunnerName: runnerName,
		Event:      &agentv1.ReportRunnerEventRequest_StartFailed{StartFailed: &agentv1.RunnerStartFailed{Error: cause.Error()}},
	}
}

func runnerCleanedEvent(commandID, runnerName string) *agentv1.ReportRunnerEventRequest {
	return &agentv1.ReportRunnerEventRequest{
		CommandId:  commandID,
		RunnerName: runnerName,
		Event:      &agentv1.ReportRunnerEventRequest_Cleaned{Cleaned: &agentv1.RunnerCleaned{}},
	}
}

// reportEvent answers one command. A failed report is logged, the cloud
// sends the command again if it never hears back.
func (s *Service) reportEvent(ctx context.Context, event *agentv1.ReportRunnerEventRequest) {
	callCtx, cancel := context.WithTimeout(ctx, reportTimeout)
	defer cancel()
	_, err := s.client.ReportRunnerEvent(callCtx, connect.NewRequest(event))
	if err != nil {
		s.logger.Warn("report runner event failed", "command_id", event.CommandId, "runner", event.RunnerName, "server_url", s.serverURL, "err", err)
	}
}

// nextReconnectDelay doubles the delay up to the maximum.
func nextReconnectDelay(current time.Duration) time.Duration {
	return min(current*2, maxReconnectDelay)
}

// withJitter spreads reconnects out so many agents do not hit the cloud at once.
func withJitter(delay time.Duration) time.Duration {
	jitter := time.Duration(rand.Int64N(int64(delay) / 2))
	return delay + jitter
}

// sleepWithContext waits for the delay and returns false when ctx ends first.
func sleepWithContext(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
