package trigger

import (
	"context"

	"github.com/actions/scaleset"
)

// JobHandler processes job lifecycle events from a trigger source.
type JobHandler interface {
	HandleJobAssigned(ctx context.Context, job *scaleset.JobAssigned) error
	HandleJobStarted(ctx context.Context, job *scaleset.JobStarted) error
	HandleJobCompleted(ctx context.Context, job *scaleset.JobCompleted) error
	ActiveJobCount() int
}

// Trigger polls or listens for job events and dispatches them to a JobHandler.
type Trigger interface {
	Run(ctx context.Context, handler JobHandler) error
}
