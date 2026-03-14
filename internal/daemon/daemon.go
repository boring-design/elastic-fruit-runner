package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/actions/scaleset"
	"github.com/actions/scaleset/listener"

	"github.com/boring-design/elastic-fruit-runner/config"
	"github.com/boring-design/elastic-fruit-runner/internal/backend"
)

// Daemon is the main controller. It registers a GitHub Actions Runner Scale
// Set, polls for job assignments, and manages ephemeral runners.
type Daemon struct {
	cfg        *config.Config
	client     *scaleset.Client
	scaleSetID int

	backend backend.Backend
	logger  *slog.Logger

	vmCounter    atomic.Int64
	activeRunner atomic.Int64
}

// New creates a Daemon from the given config and authenticated client.
func New(cfg *config.Config, client *scaleset.Client, b backend.Backend, logger *slog.Logger) *Daemon {
	return &Daemon{
		cfg:     cfg,
		client:  client,
		backend: b,
		logger:  logger,
	}
}

// Run bootstraps the scale set and starts the listener loop.
// Blocks until ctx is cancelled.
func (d *Daemon) Run(ctx context.Context) error {
	// Resolve runner group.
	group, err := d.client.GetRunnerGroupByName(ctx, d.cfg.RunnerGroup)
	if err != nil {
		return fmt.Errorf("get runner group %q: %w", d.cfg.RunnerGroup, err)
	}
	d.logger.Info("runner group resolved", "id", group.ID, "name", group.Name)

	// Desired labels for the scale set.
	desiredLabels := []scaleset.Label{
		{Name: d.cfg.ScaleSetName},
		{Name: "self-hosted"},
		{Name: "macOS"},
		{Name: "arm64"},
	}

	// Reuse existing scale set if possible, update labels if needed, or create new.
	ss, err := d.client.GetRunnerScaleSet(ctx, group.ID, d.cfg.ScaleSetName)
	if err != nil || ss == nil {
		d.logger.Info("creating scale set", "name", d.cfg.ScaleSetName)
		ss, err = d.client.CreateRunnerScaleSet(ctx, &scaleset.RunnerScaleSet{
			Name:          d.cfg.ScaleSetName,
			RunnerGroupID: group.ID,
			Labels:        desiredLabels,
		})
		if err != nil {
			return fmt.Errorf("create runner scale set: %w", err)
		}
	} else if !labelsMatch(ss.Labels, desiredLabels) {
		d.logger.Info("updating scale set labels", "id", ss.ID)
		ss, err = d.client.UpdateRunnerScaleSet(ctx, ss.ID, &scaleset.RunnerScaleSet{
			Name:          d.cfg.ScaleSetName,
			RunnerGroupID: group.ID,
			Labels:        desiredLabels,
		})
		if err != nil {
			return fmt.Errorf("update runner scale set: %w", err)
		}
	} else {
		d.logger.Info("reusing existing scale set", "id", ss.ID, "name", ss.Name)
	}
	d.scaleSetID = ss.ID
	d.logger.Info("scale set ready", "id", ss.ID, "name", ss.Name)

	// Create a message session client (handles the long-poll session lifecycle).
	msgClient, err := d.client.MessageSessionClient(ctx, ss.ID, d.cfg.ScaleSetName,
		scaleset.WithTimeout(15*time.Second),
	)
	if err != nil {
		return fmt.Errorf("create message session: %w", err)
	}
	defer msgClient.Close(context.Background())

	// Build the listener (handles long-polling and message acknowledgement).
	l, err := listener.New(msgClient, listener.Config{
		ScaleSetID: ss.ID,
		MaxRunners: d.cfg.MaxRunners,
		Logger:     d.logger,
	})
	if err != nil {
		return fmt.Errorf("create listener: %w", err)
	}

	d.logger.Info("listening for jobs",
		"scaleSet", d.cfg.ScaleSetName,
		"maxRunners", d.cfg.MaxRunners,
	)
	return l.Run(ctx, d)
}

// HandleDesiredRunnerCount implements listener.Scaler.
// Called when the number of pending jobs changes. We only spawn runners
// for the gap between what's already active and what's needed.
func (d *Daemon) HandleDesiredRunnerCount(ctx context.Context, count int) (int, error) {
	active := int(d.activeRunner.Load())
	needed := min(count, d.cfg.MaxRunners) - active
	if needed < 0 {
		needed = 0
	}

	d.logger.Info("scaling", "desired", count, "active", active, "spawning", needed)

	for i := 0; i < needed; i++ {
		go d.spawnRunner(context.Background())
	}
	return active + needed, nil
}

// HandleJobStarted implements listener.Scaler.
func (d *Daemon) HandleJobStarted(_ context.Context, job *scaleset.JobStarted) error {
	d.logger.Info("job started", "runner", job.RunnerName, "id", job.RunnerID)
	return nil
}

// HandleJobCompleted implements listener.Scaler.
func (d *Daemon) HandleJobCompleted(_ context.Context, job *scaleset.JobCompleted) error {
	d.logger.Info("job completed", "runner", job.RunnerName, "result", job.Result)
	return nil
}

// spawnRunner prepares the backend environment, registers an ephemeral runner
// with a JIT config, runs the job, then cleans up. Runs in its own goroutine.
func (d *Daemon) spawnRunner(ctx context.Context) {
	d.activeRunner.Add(1)
	defer d.activeRunner.Add(-1)

	id := d.vmCounter.Add(1)
	name := fmt.Sprintf("efr-%d-%d", time.Now().Unix(), id)
	log := d.logger.With("runner", name)

	log.Info("spawning ephemeral runner")

	defer func() {
		log.Info("cleaning up runner")
		cleanCtx := context.Background()
		d.backend.Cleanup(cleanCtx, name)
		log.Info("runner cleaned up")
	}()

	if err := d.backend.Prepare(ctx, name); err != nil {
		log.Error("prepare failed", "err", err)
		return
	}

	jitCfg, err := d.client.GenerateJitRunnerConfig(ctx,
		&scaleset.RunnerScaleSetJitRunnerSetting{Name: name},
		d.scaleSetID,
	)
	if err != nil {
		log.Error("generate JIT config failed", "err", err)
		return
	}

	if err := d.backend.RunRunner(ctx, name, jitCfg.EncodedJITConfig); err != nil {
		log.Error("runner failed", "err", err)
		return
	}

	log.Info("runner completed successfully")
}

// labelsMatch returns true if both slices contain the same label names
// (order-independent).
func labelsMatch(a, b []scaleset.Label) bool {
	if len(a) != len(b) {
		return false
	}
	set := make(map[string]struct{}, len(a))
	for _, l := range a {
		set[l.Name] = struct{}{}
	}
	for _, l := range b {
		if _, ok := set[l.Name]; !ok {
			return false
		}
	}
	return true
}
