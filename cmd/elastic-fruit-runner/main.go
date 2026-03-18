package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/actions/scaleset"
	"golang.org/x/sync/errgroup"

	"github.com/boring-design/elastic-fruit-runner/config"
	"github.com/boring-design/elastic-fruit-runner/internal/backend"
	"github.com/boring-design/elastic-fruit-runner/internal/controller"
	"github.com/boring-design/elastic-fruit-runner/internal/tracing"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	cfg := config.Load()

	if err := cfg.Validate(); err != nil {
		logger.Error("invalid configuration", "err", err)
		os.Exit(1)
	}

	var client *scaleset.Client
	var err error

	switch cfg.AuthMode() {
	case "app":
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

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	tracingShutdown, err := tracing.Setup(ctx)
	if err != nil {
		logger.Error("failed to initialize tracing", "err", err)
		os.Exit(1)
	}
	defer func() {
		if shutdownErr := tracingShutdown(context.Background()); shutdownErr != nil {
			logger.Warn("tracing shutdown error", "err", shutdownErr)
		}
	}()

	runnerSets := config.DefaultRunnerSets(cfg)

	g, gCtx := errgroup.WithContext(ctx)
	for i := range runnerSets {
		rs := &runnerSets[i]
		rsLogger := logger.With("runnerSet", rs.Name)

		var b backend.Backend
		switch rs.Backend {
		case "tart":
			b = backend.NewTartBackend(rs.Image, rsLogger)
		case "docker":
			b = backend.NewDockerBackend(rs.Image, rs.Platform, rsLogger)
		default:
			logger.Error("unknown backend", "backend", rs.Backend, "runnerSet", rs.Name)
			os.Exit(1)
		}

		d := controller.New(cfg, rs, client, b, rsLogger)

		rsLogger.Info("launching controller",
			"url", cfg.GitHubURL,
			"runnerGroup", cfg.RunnerGroup,
			"maxRunners", rs.MaxRunners,
			"image", rs.Image,
			"labels", rs.Labels,
		)

		g.Go(func() error {
			return d.Run(gCtx)
		})
	}

	if err := g.Wait(); err != nil {
		logger.Error("controller exited with error", "err", err)
		os.Exit(1)
	}

	logger.Info("shutdown complete")
}
