package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/actions/scaleset"
	"sync"
	"time"

	"github.com/boring-design/elastic-fruit-runner/config"
	"github.com/boring-design/elastic-fruit-runner/internal/backend"
	"github.com/boring-design/elastic-fruit-runner/internal/controller"
	"github.com/boring-design/elastic-fruit-runner/internal/tracing"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	cfg, err := config.Load()
	if err != nil {
		logger.Error("failed to load configuration", "err", err)
		os.Exit(1)
	}

	if err := cfg.Validate(); err != nil {
		logger.Error("invalid configuration", "err", err)
		os.Exit(1)
	}

	logger.Info("configuration loaded", cfg.RedactedSlogAttrs()...)

	var client *scaleset.Client

	switch cfg.AuthMode() {
	case "app":
		pemBytes, readErr := os.ReadFile(cfg.GitHub.App.PrivateKeyPath)
		if readErr != nil {
			logger.Error("failed to read GitHub App private key", "path", cfg.GitHub.App.PrivateKeyPath, "err", readErr)
			os.Exit(1)
		}
		logger.Info("authenticating with GitHub App", "clientID", cfg.GitHub.App.ClientID, "installationID", cfg.GitHub.App.InstallationID)
		client, err = scaleset.NewClientWithGitHubApp(scaleset.ClientWithGitHubAppConfig{
			GitHubConfigURL: cfg.GitHub.URL,
			GitHubAppAuth: scaleset.GitHubAppAuth{
				ClientID:       cfg.GitHub.App.ClientID,
				InstallationID: cfg.GitHub.App.InstallationID,
				PrivateKey:     string(pemBytes),
			},
		})
	default:
		logger.Info("authenticating with PAT")
		client, err = scaleset.NewClientWithPersonalAccessToken(
			scaleset.NewClientWithPersonalAccessTokenConfig{
				GitHubConfigURL:     cfg.GitHub.URL,
				PersonalAccessToken: cfg.GitHub.Token,
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

	var wg sync.WaitGroup
	for i := range cfg.RunnerSets {
		rs := &cfg.RunnerSets[i]
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
			"url", cfg.GitHub.URL,
			"runnerGroup", cfg.RunnerGroup,
			"maxRunners", rs.MaxRunners,
			"image", rs.Image,
			"labels", rs.Labels,
		)

		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				err := d.Run(ctx)
				if ctx.Err() != nil {
					rsLogger.Info("controller stopped", "err", err)
					return
				}
				rsLogger.Error("controller exited with error, restarting", "err", err)
				time.Sleep(5 * time.Second)
			}
		}()
	}

	wg.Wait()
	logger.Info("shutdown complete")
}
