package cloudagent

import (
	"context"

	"connectrpc.com/connect"

	agentv1 "github.com/boring-design/elastic-fruit-protocol/gen/agent/v1"
	"github.com/boring-design/elastic-fruit-runner/internal/backend"
)

// sampleQueueSize bounds how many samples wait for upload. The capture
// goroutine never blocks on the network, extra samples are dropped.
const sampleQueueSize = 256

type sampleReport struct {
	jobID      string
	runnerName string
	sample     backend.ResourceSample
}

// observeSample is called by the job store for every captured sample.
func (s *Service) observeSample(jobID string, sample backend.ResourceSample) {
	report := sampleReport{jobID: jobID, runnerName: s.jobRunnerName(jobID), sample: sample}
	select {
	case s.samples <- report:
	default:
		// Drops are expected while the stream is down, so they only warn when it is up.
		if s.streamUp.Load() {
			s.logger.Warn("resource sample dropped, upload queue is full", "job_id", jobID, "queue_size", sampleQueueSize)
		} else {
			s.logger.Debug("resource sample dropped while the command stream is down", "job_id", jobID, "queue_size", sampleQueueSize)
		}
	}
}

// runSampleSender uploads queued samples one at a time.
func (s *Service) runSampleSender(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case report := <-s.samples:
			s.sendSample(ctx, report)
		}
	}
}

func (s *Service) sendSample(ctx context.Context, report sampleReport) {
	callCtx, cancel := context.WithTimeout(ctx, reportTimeout)
	defer cancel()
	_, err := s.client.ReportResourceSamples(callCtx, connect.NewRequest(&agentv1.ReportResourceSamplesRequest{
		JobId:      report.jobID,
		RunnerName: report.runnerName,
		Samples:    []*agentv1.ResourceSample{toProtoResourceSample(report.sample)},
	}))
	if err == nil || ctx.Err() != nil {
		return
	}
	if s.streamUp.Load() {
		s.logger.Warn("report resource sample failed", "job_id", report.jobID, "runner", report.runnerName, "err", err)
	} else {
		s.logger.Debug("report resource sample failed while the command stream is down", "job_id", report.jobID, "err", err)
	}
}

func (s *Service) rememberJobRunner(jobID, runnerName string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.jobRunners[jobID] = runnerName
}

func (s *Service) forgetJobRunner(jobID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.jobRunners, jobID)
}

func (s *Service) jobRunnerName(jobID string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.jobRunners[jobID]
}
