package controller

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/actions/scaleset"
	"github.com/actions/scaleset/listener"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/boring-design/elastic-fruit-runner/internal/backend"
)

const reaperInterval = time.Minute

var _ listener.Scaler = (*ScaleSetController)(nil)

// HandleDesiredRunnerCount implements listener.Scaler.
// count is statistics.TotalAssignedJobs, which includes both jobs waiting for
// a runner and jobs already running (TotalAssignedJobs >= TotalRunningJobs).
// Runner names are registered as "preparing" before goroutines are launched,
// so a subsequent call observes the correct count immediately.
func (d *ScaleSetController) HandleDesiredRunnerCount(ctx context.Context, count int) (int, error) {
	_, span := tracer.Start(ctx, "controller.HandleDesiredRunnerCount")
	defer span.End()

	counts := d.runners.Counts()
	allocated := counts.Total()
	current := counts.Idle + counts.Busy
	needed := min(count, d.rsCfg.MaxRunners) - allocated
	if needed < 0 {
		needed = 0
	}

	span.SetAttributes(
		attribute.Int("runner.desired", count),
		attribute.Int("runner.current", current),
		attribute.Int("runner.preparing", counts.Preparing),
		attribute.Int("runner.allocated", allocated),
		attribute.Int("runner.spawning", needed),
	)
	d.logger.Info("scaling",
		"desired", count,
		"current", current,
		"preparing", counts.Preparing,
		"allocated", allocated,
		"spawning", needed,
	)

	runnerCtx := d.runners.getRunnerCtx()
	for range needed {
		name := fmt.Sprintf("%s-%s", d.rsCfg.Name, randSuffix())
		d.runners.MarkStarting(name)
		go d.startRunner(trace.ContextWithSpan(runnerCtx, span), name)
	}
	return d.runners.Count(), nil
}

// HandleJobStarted implements listener.Scaler.
// Marks the runner as busy so it is not counted as available for scaling.
func (d *ScaleSetController) HandleJobStarted(ctx context.Context, job *scaleset.JobStarted) error {
	_, span := tracer.Start(ctx, "controller.HandleJobStarted",
		trace.WithAttributes(
			attribute.String("runner.name", job.RunnerName),
			attribute.Int64("runner.id", int64(job.RunnerID)),
		),
	)
	defer span.End()

	d.runners.MarkBusy(job.RunnerName)
	diagnostics, _ := d.backend.(backend.Diagnostics)
	d.jobRecorder.RecordJobMessageStarted(d.rsCfg.Name, d.rsCfg.Backend, diagnostics, job)
	d.logger.Info("job started", "runner", job.RunnerName, "id", job.RunnerID)
	return nil
}

// HandleJobCompleted implements listener.Scaler.
// Removes the runner from tracked state and triggers async VM cleanup.
func (d *ScaleSetController) HandleJobCompleted(ctx context.Context, job *scaleset.JobCompleted) error {
	_, span := tracer.Start(ctx, "controller.HandleJobCompleted",
		trace.WithAttributes(
			attribute.String("runner.name", job.RunnerName),
			attribute.String("job.result", job.Result),
		),
	)
	defer span.End()

	name := job.RunnerName
	d.runners.Remove(name)
	d.jobRecorder.RecordJobMessageCompleted(job)
	d.logger.Info("job completed", "runner", name, "result", job.Result)

	go func() {
		cleanCtx, cleanSpan := tracer.Start(context.Background(), "controller.runner.cleanup",
			trace.WithAttributes(attribute.String("runner.name", name)),
		)
		defer cleanSpan.End()
		d.backend.Cleanup(cleanCtx, name)
	}()

	return nil
}

// startRunner is launched as a goroutine for each new runner.
// It generates a JIT config, then calls backend.Run which sets up the
// execution environment and starts the runner process. Once the runner is up
// it moves to idle state and the goroutine exits — the runner's lifecycle
// is then driven by job events.
// The caller must call runners.MarkStarting(name) before launching this goroutine.
func (d *ScaleSetController) startRunner(ctx context.Context, name string) {
	log := d.logger.With("runner", name)

	ctx, span := tracer.Start(ctx, "controller.runner.start",
		trace.WithAttributes(attribute.String("runner.name", name)),
	)
	defer span.End()

	log.Info("preparing runner")

	jitCtx, jitSpan := tracer.Start(ctx, "controller.generate_jit_config")
	jitCfg, err := d.client.GenerateJitRunnerConfig(jitCtx,
		&scaleset.RunnerScaleSetJitRunnerSetting{Name: name},
		d.scaleSetID,
	)
	jitSpan.End()
	if err != nil {
		log.Error("generate JIT config failed", "err", err)
		jitSpan.RecordError(err)
		jitSpan.SetStatus(codes.Error, "generate JIT config failed")
		span.SetStatus(codes.Error, "generate JIT config failed")
		d.runners.Remove(name)
		return
	}

	if err := d.backend.Run(ctx, name, jitCfg.EncodedJITConfig); err != nil {
		log.Error("start runner failed", "err", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, "start runner failed")
		d.runners.Remove(name)
		d.backend.Cleanup(context.Background(), name)
		d.removeGitHubRunner(context.Background(), name)
		return
	}

	d.runners.MarkIdle(name)
	log.Info("runner started, waiting for job assignment")
}

// shutdown cleans up all idle and busy runners. Called after the listener
// exits. Preparing runners are cancelled via runnerCtx and clean up themselves.
func (d *ScaleSetController) shutdown(ctx context.Context) {
	d.logger.Info("shutting down, cleaning up runners")

	toCleanup, preparingCount := d.runners.removeAll()

	if preparingCount > 0 {
		d.logger.Info("aborting in-flight preparations", "count", preparingCount)
	}

	for _, name := range toCleanup {
		d.logger.Info("cleaning up runner on shutdown", "runner", name)
		d.backend.Cleanup(ctx, name)
		d.removeGitHubRunner(ctx, name)
	}
}

// runIdleReaper periodically evicts idle runners that have exceeded the idle
// timeout, mirroring the keepAliveTime behaviour of ThreadPoolExecutor.
func (d *ScaleSetController) runIdleReaper(ctx context.Context) {
	ticker := time.NewTicker(reaperInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			d.reapExpiredIdleRunners()
		}
	}
}

func (d *ScaleSetController) reapExpiredIdleRunners() {
	now := time.Now()
	timeout := d.idleTimeout

	expired := d.runners.removeIdleOlderThan(now, timeout)

	for _, name := range expired {
		d.logger.Info("idle runner timed out, cleaning up", "runner", name, "idleTimeout", timeout)
		go func(n string) {
			cleanCtx, cleanSpan := tracer.Start(context.Background(), "controller.runner.cleanup",
				trace.WithAttributes(
					attribute.String("runner.name", n),
					attribute.String("cleanup.reason", "idle_timeout"),
				),
			)
			defer cleanSpan.End()
			d.backend.Cleanup(cleanCtx, n)
			d.removeGitHubRunner(cleanCtx, n)
		}(name)
	}
}

// removeGitHubRunner deregisters a runner from GitHub by name.
// Best-effort: logs errors but does not propagate them, since the
// backend resource is already being cleaned up.
func (d *ScaleSetController) removeGitHubRunner(ctx context.Context, name string) {
	runner, err := d.client.GetRunnerByName(ctx, name)
	if err != nil || runner == nil {
		return
	}
	if err := d.client.RemoveRunner(ctx, int64(runner.ID)); err != nil {
		d.logger.Warn("failed to remove runner from GitHub", "runner", name, "err", err)
	}
}

// randSuffix returns a short random hex string (5 chars), similar to
// Kubernetes Deployment pod suffixes.
func randSuffix() string {
	var b [3]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])[:5]
}

// GetRunnerSetInfo returns the static configuration of this runner set.
func (d *ScaleSetController) GetRunnerSetInfo() RunnerSetInfo {
	labels := make([]string, len(d.rsCfg.Labels))
	copy(labels, d.rsCfg.Labels)
	return RunnerSetInfo{
		Name:       d.rsCfg.Name,
		Backend:    d.rsCfg.Backend,
		Image:      d.rsCfg.Image,
		Labels:     labels,
		MaxRunners: d.rsCfg.MaxRunners,
	}
}

// GetScope returns the org/repo scope of this runner set.
func (d *ScaleSetController) GetScope() string {
	return d.scope
}

// IsConnected returns whether the controller is connected to GitHub.
func (d *ScaleSetController) IsConnected() bool {
	return d.connected.Load()
}

// GetRunners returns a point-in-time copy of all runners and their states.
func (d *ScaleSetController) GetRunners() []RunnerSnapshot {
	return d.runners.Snapshot()
}
