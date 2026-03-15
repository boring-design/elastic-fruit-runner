package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/actions/scaleset"

	"github.com/boring-design/elastic-fruit-runner/config"
	"github.com/boring-design/elastic-fruit-runner/internal/backend"
	"github.com/boring-design/elastic-fruit-runner/internal/runnerpool"
	"github.com/boring-design/elastic-fruit-runner/internal/scheduler"
	"github.com/boring-design/elastic-fruit-runner/internal/trigger"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	cfg := config.Load()

	if cfg.GitHubURL == "" {
		logger.Error("GitHub config URL is required: set --url flag or GITHUB_CONFIG_URL env var")
		os.Exit(1)
	}

	var client *scaleset.Client
	var err error

	switch cfg.AuthMode() {
	case "app":
		if cfg.AppInstallationID == 0 {
			logger.Error("GitHub App auth requires --app-installation-id or GITHUB_APP_INSTALLATION_ID")
			os.Exit(1)
		}
		if cfg.AppPrivateKeyPath == "" {
			logger.Error("GitHub App auth requires --app-private-key or GITHUB_APP_PRIVATE_KEY_PATH")
			os.Exit(1)
		}
		pemBytes, readErr := os.ReadFile(cfg.AppPrivateKeyPath)
		if readErr != nil {
			logger.Error("failed to read GitHub App private key", "path", cfg.AppPrivateKeyPath, "err", readErr)
			os.Exit(1)
		}
		logger.Info("authenticating with GitHub App", "clientID", cfg.AppClientID, "installationID", cfg.AppInstallationID)
		client, err = scaleset.NewClientWithGitHubApp(scaleset.ClientWithGitHubAppConfig{
			GitHubConfigURL: cfg.GitHubURL,
			GitHubAppAuth: scaleset.GitHubAppAuth{
				ClientID:       cfg.AppClientID,
				InstallationID: cfg.AppInstallationID,
				PrivateKey:     string(pemBytes),
			},
		})
	default:
		if cfg.GitHubToken == "" {
			logger.Error("no auth configured: set --token (PAT) or --app-client-id (GitHub App)")
			os.Exit(1)
		}
		logger.Info("authenticating with PAT")
		client, err = scaleset.NewClientWithPersonalAccessToken(
			scaleset.NewClientWithPersonalAccessTokenConfig{
				GitHubConfigURL:     cfg.GitHubURL,
				PersonalAccessToken: cfg.GitHubToken,
			},
		)
	}

	if err != nil {
		logger.Error("failed to create scale set client", "err", err)
		os.Exit(1)
	}

	// Select backend based on mode.
	var b backend.Backend
	switch cfg.Mode {
	case "host":
		b = backend.NewHostBackend(logger)
		logger.Info("using host backend (runner runs directly on this machine)")
	case "tart":
		b = backend.NewTartBackend(cfg.VMImage, logger)
	default:
		logger.Error("unknown mode — use 'host' or 'tart'", "mode", cfg.Mode)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	logger.Info("elastic-fruit-runner starting",
		"url", cfg.GitHubURL,
		"mode", cfg.Mode,
		"scaleSet", cfg.ScaleSetName,
		"runnerGroup", cfg.RunnerGroup,
		"maxRunners", cfg.MaxRunners,
		"poolSize", cfg.PoolSize,
	)

	if err := run(ctx, cfg, client, b, logger); err != nil && !errors.Is(err, context.Canceled) {
		logger.Error("daemon exited with error", "err", err)
		os.Exit(1)
	}

	logger.Info("shutdown complete")
}

func run(ctx context.Context, cfg *config.Config, client *scaleset.Client, b backend.Backend, logger *slog.Logger) error {
	// Bootstrap scale set.
	group, err := client.GetRunnerGroupByName(ctx, cfg.RunnerGroup)
	if err != nil {
		return fmt.Errorf("get runner group %q: %w", cfg.RunnerGroup, err)
	}
	logger.Info("runner group resolved", "id", group.ID, "name", group.Name)

	desiredLabels := []scaleset.Label{
		{Name: cfg.ScaleSetName},
		{Name: "self-hosted"},
		{Name: "macOS"},
		{Name: "arm64"},
	}

	ss, err := client.GetRunnerScaleSet(ctx, group.ID, cfg.ScaleSetName)
	if err != nil || ss == nil {
		logger.Info("creating scale set", "name", cfg.ScaleSetName)
		ss, err = client.CreateRunnerScaleSet(ctx, &scaleset.RunnerScaleSet{
			Name:          cfg.ScaleSetName,
			RunnerGroupID: group.ID,
			Labels:        desiredLabels,
		})
		if err != nil {
			return fmt.Errorf("create runner scale set: %w", err)
		}
	} else if !labelsMatch(ss.Labels, desiredLabels) {
		logger.Info("updating scale set labels", "id", ss.ID)
		ss, err = client.UpdateRunnerScaleSet(ctx, ss.ID, &scaleset.RunnerScaleSet{
			Name:          cfg.ScaleSetName,
			RunnerGroupID: group.ID,
			Labels:        desiredLabels,
		})
		if err != nil {
			return fmt.Errorf("update runner scale set: %w", err)
		}
	} else {
		logger.Info("reusing existing scale set", "id", ss.ID, "name", ss.Name)
	}
	logger.Info("scale set ready", "id", ss.ID, "name", ss.Name)

	// Create message session.
	msgClient, err := client.MessageSessionClient(ctx, ss.ID, cfg.ScaleSetName)
	if err != nil {
		return fmt.Errorf("create message session: %w", err)
	}
	defer msgClient.Close(context.Background())

	// Layer 1: RunnerPool
	pool := runnerpool.New(cfg.PoolSize, b, logger)
	go pool.Start(ctx)

	// Layer 2: Scheduler
	sched := scheduler.New(pool, client, ss.ID, logger)
	sched.StartCleanup(ctx)

	// Layer 3: Trigger
	t := trigger.NewScaleSetTrigger(msgClient, ss.ID, cfg.MaxRunners, logger)

	logger.Info("listening for jobs",
		"scaleSet", cfg.ScaleSetName,
		"maxRunners", cfg.MaxRunners,
	)

	return t.Run(ctx, sched)
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
