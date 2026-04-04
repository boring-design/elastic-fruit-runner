package controller

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/actions/scaleset"
	"github.com/actions/scaleset/listener"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel"

	"github.com/boring-design/elastic-fruit-runner/config"
	"github.com/boring-design/elastic-fruit-runner/internal/backend"
	"github.com/boring-design/elastic-fruit-runner/internal/registry"
)

var tracer = otel.Tracer("github.com/boring-design/elastic-fruit-runner/internal/controller")

// Version and CommitSHA are set at build time via -ldflags.
var (
	Version   = "dev"
	CommitSHA = "unknown"
)

// ScaleSetController registers a GitHub Actions Runner Scale Set, polls for
// job assignments via the listener, and manages the lifecycle of ephemeral
// runners that run each job.
type ScaleSetController struct {
	rsCfg       *config.RunnerSetConfig
	runnerGroup string
	idleTimeout time.Duration

	client     *scaleset.Client
	scaleSetID int

	backend  backend.Backend
	logger   *slog.Logger
	registry *registry.Registry

	runners runnerState

	// runnerCancel cancels the context used by startRunner goroutines,
	// allowing in-flight VM preparations to be aborted on shutdown.
	runnerCancel context.CancelFunc
}

// New creates a ScaleSetController for a single runner set.
func New(rsCfg *config.RunnerSetConfig, runnerGroup string, idleTimeout time.Duration, client *scaleset.Client, b backend.Backend, reg *registry.Registry) *ScaleSetController {
	return &ScaleSetController{
		rsCfg:       rsCfg,
		runnerGroup: runnerGroup,
		idleTimeout: idleTimeout,
		client:      client,
		backend:     b,
		registry:    reg,
		logger:      slog.Default().With("runnerSet", rsCfg.Name),
	}
}

// Run bootstraps the scale set and starts the listener loop.
// Blocks until ctx is cancelled.
func (d *ScaleSetController) Run(ctx context.Context) error {
	runnerCtx, runnerCancel := context.WithCancel(ctx)
	d.runnerCancel = runnerCancel

	d.logger.Info("cleaning up resources from previous runs")
	d.backend.CleanupAll(ctx, d.rsCfg.Name)

	group, err := d.client.GetRunnerGroupByName(ctx, d.runnerGroup)
	if err != nil {
		runnerCancel()
		return fmt.Errorf("get runner group %q: %w", d.runnerGroup, err)
	}
	d.logger.Info("runner group resolved", "id", group.ID, "name", group.Name)

	desiredLabels := make([]scaleset.Label, 0, len(d.rsCfg.Labels)+1)
	desiredLabels = append(desiredLabels, scaleset.Label{Name: d.rsCfg.Name})
	for _, l := range d.rsCfg.Labels {
		desiredLabels = append(desiredLabels, scaleset.Label{Name: l})
	}

	ss, err := d.client.GetRunnerScaleSet(ctx, group.ID, d.rsCfg.Name)
	switch {
	case err != nil || ss == nil:
		d.logger.Info("creating scale set", "name", d.rsCfg.Name)
		ss, err = d.client.CreateRunnerScaleSet(ctx, &scaleset.RunnerScaleSet{
			Name:          d.rsCfg.Name,
			RunnerGroupID: group.ID,
			Labels:        desiredLabels,
		})
		if err != nil {
			runnerCancel()
			return fmt.Errorf("create runner scale set: %w", err)
		}
	case !labelsMatch(ss.Labels, desiredLabels):
		d.logger.Info("updating scale set labels", "id", ss.ID)
		ss, err = d.client.UpdateRunnerScaleSet(ctx, ss.ID, &scaleset.RunnerScaleSet{
			Name:          d.rsCfg.Name,
			RunnerGroupID: group.ID,
			Labels:        desiredLabels,
		})
		if err != nil {
			runnerCancel()
			return fmt.Errorf("update runner scale set: %w", err)
		}
	default:
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
	hostname = fmt.Sprintf("%s-%s", hostname, d.rsCfg.Name)

	msgClient, err := d.client.MessageSessionClient(ctx, ss.ID, hostname)
	if err != nil {
		runnerCancel()
		return fmt.Errorf("create message session: %w", err)
	}
	defer msgClient.Close(context.Background())

	l, err := listener.New(msgClient, listener.Config{
		ScaleSetID: ss.ID,
		MaxRunners: d.rsCfg.MaxRunners,
		Logger:     d.logger,
	})
	if err != nil {
		runnerCancel()
		return fmt.Errorf("create listener: %w", err)
	}

	// runnerCtx is stored so that startRunner goroutines can use it.
	// We embed it as a closure rather than a field to keep Run re-entrant.
	d.runners.setRunnerCtx(runnerCtx)
	go d.runIdleReaper(runnerCtx)

	d.logger.Info("listening for jobs",
		"scaleSet", d.rsCfg.Name,
		"maxRunners", d.rsCfg.MaxRunners,
	)

	d.registry.SetConnected(d.rsCfg.Name, true)
	listenerErr := l.Run(ctx, d)
	d.registry.SetConnected(d.rsCfg.Name, false)

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
