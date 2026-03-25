package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"sync"
	"time"

	"github.com/actions/scaleset"

	"github.com/boring-design/elastic-fruit-runner/config"
	"github.com/boring-design/elastic-fruit-runner/internal/backend"
	"github.com/boring-design/elastic-fruit-runner/internal/controller"
	"github.com/boring-design/elastic-fruit-runner/internal/tracing"
)

func main() {
	if err := run(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run() error {
	bootstrapLogger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	logLevel, err := cfg.ParsedLogLevel()
	if err != nil {
		bootstrapLogger.Error("invalid log level", "configured", cfg.LogLevel, "valid_values", "debug, info, warn, error", "err", err)
		return fmt.Errorf("invalid log level %q: %w", cfg.LogLevel, err)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	}))

	logger.Info("configuration loaded", cfg.RedactedSlogAttrs()...)

	var client *scaleset.Client

	switch cfg.AuthMode() {
	case "app":
		pemBytes, readErr := os.ReadFile(cfg.GitHub.App.PrivateKeyPath)
		if readErr != nil {
			return fmt.Errorf("read GitHub App private key %s: %w", cfg.GitHub.App.PrivateKeyPath, readErr)
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
		return fmt.Errorf("create scale set client: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	tracingShutdown, err := tracing.Setup(ctx)
	if err != nil {
		return fmt.Errorf("initialize tracing: %w", err)
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
			return fmt.Errorf("unknown backend %q for runner set %q", rs.Backend, rs.Name)
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
	return nil
}
