package trigger

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/actions/scaleset"
)

// MessageClient is the subset of scaleset.MessageSessionClient used by ScaleSetTrigger.
type MessageClient interface {
	GetMessage(ctx context.Context, lastMessageID, maxCapacity int) (*scaleset.RunnerScaleSetMessage, error)
	DeleteMessage(ctx context.Context, messageID int) error
	Session() scaleset.RunnerScaleSetSession
}

// ScaleSetTrigger polls the GitHub Actions scale set message queue and
// dispatches job events to a JobHandler. It replaces listener.Listener
// to preserve per-job information (JobID) needed for deduplication.
type ScaleSetTrigger struct {
	msgClient  MessageClient
	scaleSetID int
	maxRunners int
	logger     *slog.Logger
}

// NewScaleSetTrigger creates a trigger backed by the scale set message queue.
func NewScaleSetTrigger(msgClient MessageClient, scaleSetID, maxRunners int, logger *slog.Logger) *ScaleSetTrigger {
	return &ScaleSetTrigger{
		msgClient:  msgClient,
		scaleSetID: scaleSetID,
		maxRunners: maxRunners,
		logger:     logger,
	}
}

// Run polls for messages and dispatches them to the handler.
// Blocks until ctx is cancelled.
func (t *ScaleSetTrigger) Run(ctx context.Context, handler JobHandler) error {
	lastMessageID := 0

	// Log initial session statistics.
	session := t.msgClient.Session()
	if session.Statistics != nil {
		stats := session.Statistics
		t.logger.Info("initial session statistics",
			"totalAssignedJobs", stats.TotalAssignedJobs,
			"totalRunningJobs", stats.TotalRunningJobs,
			"totalAvailableJobs", stats.TotalAvailableJobs,
		)
		if stats.TotalAssignedJobs > 0 {
			t.logger.Warn("jobs assigned before this session started; they will be re-delivered via GetMessage if still pending")
		}
	}

	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		maxCapacity := max(t.maxRunners-handler.ActiveJobCount(), 0)

		msg, err := t.msgClient.GetMessage(ctx, lastMessageID, maxCapacity)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("get message: %w", err)
		}

		if msg == nil {
			// No messages; the long-poll timed out. Loop and retry.
			continue
		}

		// Acknowledge the message before processing to avoid redelivery.
		if err := t.msgClient.DeleteMessage(ctx, msg.MessageID); err != nil {
			t.logger.Error("failed to delete message", "messageID", msg.MessageID, "err", err)
		}

		lastMessageID = msg.MessageID

		// Dispatch job events.
		for _, job := range msg.JobAssignedMessages {
			if err := handler.HandleJobAssigned(ctx, job); err != nil {
				t.logger.Error("handle job assigned failed", "jobID", job.JobID, "err", err)
			}
		}

		for _, job := range msg.JobStartedMessages {
			if err := handler.HandleJobStarted(ctx, job); err != nil {
				t.logger.Error("handle job started failed", "jobID", job.JobID, "err", err)
			}
		}

		for _, job := range msg.JobCompletedMessages {
			if err := handler.HandleJobCompleted(ctx, job); err != nil {
				t.logger.Error("handle job completed failed", "jobID", job.JobID, "err", err)
			}
		}
	}
}
