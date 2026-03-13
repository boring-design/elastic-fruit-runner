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
	"github.com/boring-design/elastic-fruit-runner/internal/runner"
	"github.com/boring-design/elastic-fruit-runner/internal/tart"
)

// Daemon is the main controller. It registers a GitHub Actions Runner Scale
// Set, polls for job assignments, and manages ephemeral Tart VMs.
type Daemon struct {
	cfg        *config.Config
	client     *scaleset.Client
	scaleSetID int

	tart      *tart.Manager
	registrar *runner.Registrar
	logger    *slog.Logger

	vmCounter atomic.Int64
}

// New creates a Daemon from the given config and authenticated client.
func New(cfg *config.Config, client *scaleset.Client, logger *slog.Logger) *Daemon {
	t := tart.NewManager(logger)
	return &Daemon{
		cfg:       cfg,
		client:    client,
		tart:      t,
		registrar: runner.NewRegistrar(t, logger),
		logger:    logger,
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

	// Get existing scale set or create one.
	ss, err := d.client.GetRunnerScaleSet(ctx, group.ID, d.cfg.ScaleSetName)
	if err != nil {
		d.logger.Info("scale set not found — creating", "name", d.cfg.ScaleSetName)
		ss, err = d.client.CreateRunnerScaleSet(ctx, &scaleset.RunnerScaleSet{
			Name:          d.cfg.ScaleSetName,
			RunnerGroupID: group.ID,
			Labels: []scaleset.Label{
				{Name: "self-hosted"},
				{Name: "macOS"},
				{Name: "arm64"},
			},
		})
		if err != nil {
			return fmt.Errorf("create runner scale set: %w", err)
		}
	}
	d.scaleSetID = ss.ID
	d.logger.Info("scale set ready", "id", ss.ID, "name", ss.Name)

	// Create a message session client (handles the long-poll session lifecycle).
	msgClient, err := d.client.MessageSessionClient(ctx, ss.ID, d.cfg.ScaleSetName)
	if err != nil {
		return fmt.Errorf("create message session: %w", err)
	}
	defer msgClient.Close(ctx)

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
// Called when the number of pending jobs changes. We spawn one VM per desired
// runner, up to the configured maximum.
func (d *Daemon) HandleDesiredRunnerCount(ctx context.Context, count int) (int, error) {
	actual := min(count, d.cfg.MaxRunners)
	d.logger.Info("scaling", "desired", count, "spawning", actual)

	for i := 0; i < actual; i++ {
		go d.spawnRunner(context.Background())
	}
	return actual, nil
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

// spawnRunner clones a Tart VM, registers an ephemeral runner with a JIT
// config, runs the job, then tears down the VM. Runs in its own goroutine.
func (d *Daemon) spawnRunner(ctx context.Context) {
	id := d.vmCounter.Add(1)
	vmName := fmt.Sprintf("efr-%d-%d", time.Now().Unix(), id)
	log := d.logger.With("vm", vmName)

	log.Info("spawning ephemeral runner VM")

	defer func() {
		log.Info("cleaning up VM")
		cleanCtx := context.Background()
		if err := d.tart.Stop(cleanCtx, vmName); err != nil {
			log.Warn("stop VM", "err", err)
		}
		if err := d.tart.Delete(cleanCtx, vmName); err != nil {
			log.Warn("delete VM", "err", err)
		}
		log.Info("VM removed")
	}()

	if err := d.tart.Clone(ctx, d.cfg.VMImage, vmName); err != nil {
		log.Error("clone VM failed", "err", err)
		return
	}

	if err := d.tart.Start(ctx, vmName); err != nil {
		log.Error("start VM failed", "err", err)
		return
	}

	if _, err := d.tart.IPAddress(ctx, vmName); err != nil {
		log.Error("VM unreachable", "err", err)
		return
	}

	jitCfg, err := d.client.GenerateJitRunnerConfig(ctx,
		&scaleset.RunnerScaleSetJitRunnerSetting{Name: vmName},
		d.scaleSetID,
	)
	if err != nil {
		log.Error("generate JIT config failed", "err", err)
		return
	}

	if err := d.registrar.StartWithJITConfig(ctx, vmName, jitCfg.EncodedJITConfig); err != nil {
		log.Error("runner failed", "err", err)
		return
	}

	log.Info("runner completed successfully")
}
