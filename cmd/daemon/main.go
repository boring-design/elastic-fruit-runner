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
	"github.com/boring-design/elastic-fruit-runner/internal/daemon"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	cfg := config.Load()

	if cfg.GitHubToken == "" {
		logger.Error("GitHub token is required: set --token flag or GITHUB_TOKEN env var")
		os.Exit(1)
	}
	if cfg.GitHubURL == "" {
		logger.Error("GitHub config URL is required: set --url flag or GITHUB_CONFIG_URL env var")
		os.Exit(1)
	}

	client, err := scaleset.NewClientWithPersonalAccessToken(
		scaleset.NewClientWithPersonalAccessTokenConfig{
			GitHubConfigURL:     cfg.GitHubURL,
			PersonalAccessToken: cfg.GitHubToken,
		},
	)
	if err != nil {
		logger.Error("failed to create scale set client", "err", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	d := daemon.New(cfg, client, logger)

	logger.Info("elastic-fruit-runner starting",
		"url", cfg.GitHubURL,
		"scaleSet", cfg.ScaleSetName,
		"runnerGroup", cfg.RunnerGroup,
		"vmImage", cfg.VMImage,
		"maxRunners", cfg.MaxRunners,
	)

	if err := d.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		logger.Error("daemon exited with error", "err", err)
		os.Exit(1)
	}

	logger.Info("shutdown complete")
}
