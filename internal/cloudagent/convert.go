package cloudagent

import (
	"time"

	"github.com/actions/scaleset"
	"google.golang.org/protobuf/types/known/timestamppb"

	agentv1 "github.com/boring-design/elastic-fruit-runner/gen/agent/v1"
	"github.com/boring-design/elastic-fruit-runner/internal/backend"
	"github.com/boring-design/elastic-fruit-runner/internal/controller"
	"github.com/boring-design/elastic-fruit-runner/internal/probe"
	"github.com/boring-design/elastic-fruit-runner/internal/vitals"
)

// jobResultString maps the cloud result enum to the strings the job store
// knows. The second value is false for an unknown enum value.
func jobResultString(result agentv1.JobResult) (string, bool) {
	switch result {
	case agentv1.JobResult_JOB_RESULT_SUCCEEDED:
		return "succeeded", true
	case agentv1.JobResult_JOB_RESULT_FAILED:
		return "failed", true
	case agentv1.JobResult_JOB_RESULT_CANCELED:
		return "canceled", true
	default:
		return "", false
	}
}

func toProtoRunnerState(state controller.RunnerState) agentv1.RunnerState {
	switch state {
	case controller.StatePreparing:
		return agentv1.RunnerState_RUNNER_STATE_PREPARING
	case controller.StateIdle:
		return agentv1.RunnerState_RUNNER_STATE_IDLE
	case controller.StateBusy:
		return agentv1.RunnerState_RUNNER_STATE_BUSY
	default:
		return agentv1.RunnerState_RUNNER_STATE_UNSPECIFIED
	}
}

func toProtoHostVitals(value vitals.Vitals) *agentv1.HostVitals {
	return &agentv1.HostVitals{
		CpuUsagePercent:      value.CPUUsagePercent,
		MemoryUsagePercent:   value.MemoryUsagePercent,
		DiskUsagePercent:     value.DiskUsagePercent,
		TemperatureCelsius:   value.TemperatureCelsius,
		LoadOne:              value.LoadOne,
		MemoryUsedBytes:      value.MemoryUsedBytes,
		MemoryAvailableBytes: value.MemoryAvailableBytes,
		SwapUsedBytes:        value.SwapUsedBytes,
		DiskUsedBytes:        value.DiskUsedBytes,
		DiskAvailableBytes:   value.DiskAvailableBytes,
		DiskReadBytes:        value.DiskReadBytes,
		DiskWriteBytes:       value.DiskWriteBytes,
	}
}

func toProtoResourceSample(sample backend.ResourceSample) *agentv1.ResourceSample {
	accuracy := agentv1.ResourceAccuracy_RESOURCE_ACCURACY_EXACT
	if sample.Accuracy == "estimate" {
		accuracy = agentv1.ResourceAccuracy_RESOURCE_ACCURACY_ESTIMATE
	}
	return &agentv1.ResourceSample{
		RecordedAt:           timestamppb.New(sample.RecordedAt),
		Source:               sample.Source,
		Accuracy:             accuracy,
		CpuPercent:           sample.CPUPercent,
		MemoryUsedBytes:      sample.MemoryUsedBytes,
		MemoryAvailableBytes: sample.MemoryAvailableBytes,
		DiskUsedBytes:        sample.DiskUsedBytes,
		DiskAvailableBytes:   sample.DiskAvailableBytes,
		DiskReadBytes:        sample.DiskReadBytes,
		DiskWriteBytes:       sample.DiskWriteBytes,
		NetworkReceiveBytes:  sample.NetworkReceiveBytes,
		NetworkSendBytes:     sample.NetworkSendBytes,
	}
}

func toProtoBackendCapabilities(results []probe.BackendResult) []*agentv1.BackendCapability {
	capabilities := make([]*agentv1.BackendCapability, 0, len(results))
	for _, result := range results {
		capabilities = append(capabilities, &agentv1.BackendCapability{
			Backend: result.Backend,
			Version: result.Version,
		})
	}
	return capabilities
}

// toJobStarted shapes a cloud job assignment like the GitHub message the
// standalone controller receives, so the job store records both the same way.
func toJobStarted(assigned *agentv1.JobAssigned) *scaleset.JobStarted {
	job := assigned.Job
	return &scaleset.JobStarted{
		RunnerName: assigned.RunnerName,
		JobMessageBase: scaleset.JobMessageBase{
			JobID:              job.JobId,
			WorkflowRunID:      job.WorkflowRunId,
			JobWorkflowRef:     job.WorkflowRef,
			JobDisplayName:     job.DisplayName,
			OwnerName:          job.Owner,
			RepositoryName:     job.Repository,
			EventName:          job.EventName,
			RequestLabels:      job.Labels,
			QueueTime:          timeOrZero(job.QueuedAt),
			ScaleSetAssignTime: timeOrZero(job.ScaleSetAssignedAt),
			RunnerAssignTime:   timeOrZero(job.RunnerAssignedAt),
		},
	}
}

func toJobCompleted(finished *agentv1.JobFinished, result string) *scaleset.JobCompleted {
	return &scaleset.JobCompleted{
		Result:     result,
		RunnerName: finished.RunnerName,
		JobMessageBase: scaleset.JobMessageBase{
			JobID:      finished.JobId,
			FinishTime: timeOrZero(finished.FinishedAt),
		},
	}
}

func timeOrZero(value *timestamppb.Timestamp) (result time.Time) {
	if value == nil {
		return result
	}
	return value.AsTime()
}
