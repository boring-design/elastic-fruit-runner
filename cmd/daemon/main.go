package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/actions/scaleset"

	"github.com/boring-design/elastic-fruit-runner/config"
	"github.com/boring-design/elastic-fruit-runner/internal/backend"
	"github.com/boring-design/elastic-fruit-runner/internal/daemon"
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

	d := daemon.New(cfg, client, b, logger)

	logger.Info("elastic-fruit-runner starting",
		"url", cfg.GitHubURL,
		"mode", cfg.Mode,
		"scaleSet", cfg.ScaleSetName,
		"runnerGroup", cfg.RunnerGroup,
		"maxRunners", cfg.MaxRunners,
	)

	if err := d.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		logger.Error("daemon exited with error", "err", err)
		os.Exit(1)
	}

	logger.Info("shutdown complete")
}
