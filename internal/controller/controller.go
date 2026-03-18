package controller

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync/atomic"

	"github.com/actions/scaleset"
	"github.com/actions/scaleset/listener"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel"

	"github.com/boring-design/elastic-fruit-runner/config"
	"github.com/boring-design/elastic-fruit-runner/internal/backend"
)

var tracer = otel.Tracer("github.com/boring-design/elastic-fruit-runner/internal/controller")

// Version and CommitSHA are set at build time via -ldflags.
var (
	Version   = "dev"
	CommitSHA = "unknown"
)

// ScaleSetController registers a GitHub Actions Runner Scale Set, polls for
// job assignments via the listener, and manages the lifecycle of ephemeral
// Tart VMs that run each job.
type ScaleSetController struct {
	cfg        *config.Config
	client     *scaleset.Client
	scaleSetID int

	backend backend.Backend
	logger  *slog.Logger

	vmCounter atomic.Int64
	runners   runnerState

	// runnerCancel cancels the context used by prepareAndStart goroutines,
	// allowing in-flight VM preparations to be aborted on shutdown.
	runnerCancel context.CancelFunc
}

// New creates a ScaleSetController from the given config and authenticated client.
func New(cfg *config.Config, client *scaleset.Client, b backend.Backend, logger *slog.Logger) *ScaleSetController {
	return &ScaleSetController{
		cfg:     cfg,
		client:  client,
		backend: b,
		logger:  logger,
	}
}

// Run bootstraps the scale set and starts the listener loop.
// Blocks until ctx is cancelled.
func (d *ScaleSetController) Run(ctx context.Context) error {
	runnerCtx, runnerCancel := context.WithCancel(ctx)
	d.runnerCancel = runnerCancel

	group, err := d.client.GetRunnerGroupByName(ctx, d.cfg.RunnerGroup)
	if err != nil {
		runnerCancel()
		return fmt.Errorf("get runner group %q: %w", d.cfg.RunnerGroup, err)
	}
	d.logger.Info("runner group resolved", "id", group.ID, "name", group.Name)

	desiredLabels := []scaleset.Label{
		{Name: d.cfg.ScaleSetName},
		{Name: "self-hosted"},
		{Name: "macOS"},
		{Name: "arm64"},
	}

	ss, err := d.client.GetRunnerScaleSet(ctx, group.ID, d.cfg.ScaleSetName)
	if err != nil || ss == nil {
		d.logger.Info("creating scale set", "name", d.cfg.ScaleSetName)
		ss, err = d.client.CreateRunnerScaleSet(ctx, &scaleset.RunnerScaleSet{
			Name:          d.cfg.ScaleSetName,
			RunnerGroupID: group.ID,
			Labels:        desiredLabels,
		})
		if err != nil {
			runnerCancel()
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
			runnerCancel()
			return fmt.Errorf("update runner scale set: %w", err)
		}
	} else {
		d.logger.Info("reusing existing scale set", "id", ss.ID, "name", ss.Name)
	}
	d.scaleSetID = ss.ID
	d.logger.Info("scale set ready", "id", ss.ID, "name", ss.Name)

	d.client.SetSystemInfo(scaleset.SystemInfo{
		System:     "elastic-fruit-runner",
		Subsystem:  "controller",
		Version:    Version,
		CommitSHA:  CommitSHA,
		ScaleSetID: ss.ID,
	})

	defer func() {
		d.logger.Info("deleting runner scale set", "id", ss.ID)
		if err := d.client.DeleteRunnerScaleSet(context.WithoutCancel(ctx), ss.ID); err != nil {
			d.logger.Error("failed to delete runner scale set", "id", ss.ID, "err", err)
		}
	}()

	hostname, err := os.Hostname()
	if err != nil {
		hostname = uuid.NewString()
		d.logger.Info("failed to get hostname, using uuid fallback", "uuid", hostname, "err", err)
	}

	msgClient, err := d.client.MessageSessionClient(ctx, ss.ID, hostname)
	if err != nil {
		runnerCancel()
		return fmt.Errorf("create message session: %w", err)
	}
	defer msgClient.Close(context.Background())

	l, err := listener.New(msgClient, listener.Config{
		ScaleSetID: ss.ID,
		MaxRunners: d.cfg.MaxRunners,
		Logger:     d.logger,
	})
	if err != nil {
		runnerCancel()
		return fmt.Errorf("create listener: %w", err)
	}

	// runnerCtx is stored so that prepareAndStart goroutines can use it.
	// We embed it as a closure rather than a field to keep Run re-entrant.
	d.runners.setRunnerCtx(runnerCtx)
	go d.runIdleReaper(runnerCtx)

	d.logger.Info("listening for jobs",
		"scaleSet", d.cfg.ScaleSetName,
		"maxRunners", d.cfg.MaxRunners,
	)

	listenerErr := l.Run(ctx, d)

	// Stop in-flight preparations and clean up all remaining runners.
	runnerCancel()
	d.shutdown(context.WithoutCancel(ctx))

	return listenerErr
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
