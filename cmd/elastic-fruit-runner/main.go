package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/actions/scaleset"

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

	for i := range cfg.Orgs {
		org := &cfg.Orgs[i]
		client, clientErr := createClient(org.ConfigURL(), &org.Auth, logger)
		if clientErr != nil {
			logger.Error("failed to create client for org", "org", org.Org, "err", clientErr)
			os.Exit(1)
		}

		for j := range org.RunnerSets {
			rs := &org.RunnerSets[j]
			if err := launchController(&wg, ctx, rs, org.RunnerGroup, cfg.IdleTimeout, client, logger); err != nil {
				logger.Error("failed to launch controller", "runnerSet", rs.Name, "err", err)
				os.Exit(1)
			}
		}
	}

	for i := range cfg.Repos {
		repo := &cfg.Repos[i]
		client, clientErr := createClient(repo.ConfigURL(), &repo.Auth, logger)
		if clientErr != nil {
			logger.Error("failed to create client for repo", "repo", repo.Repo, "err", clientErr)
			os.Exit(1)
		}

		for j := range repo.RunnerSets {
			rs := &repo.RunnerSets[j]
			if err := launchController(&wg, ctx, rs, "", cfg.IdleTimeout, client, logger); err != nil {
				logger.Error("failed to launch controller", "runnerSet", rs.Name, "err", err)
				os.Exit(1)
			}
		}
	}

	wg.Wait()
	logger.Info("shutdown complete")
}

func createClient(configURL string, auth *config.AuthConfig, logger *slog.Logger) (*scaleset.Client, error) {
	switch auth.Mode() {
	case "app":
		pemBytes, readErr := os.ReadFile(auth.GitHubApp.PrivateKeyPath)
		if readErr != nil {
			return nil, fmt.Errorf("read GitHub App private key %s: %w", auth.GitHubApp.PrivateKeyPath, readErr)
		}
		logger.Info("authenticating with GitHub App",
			"configURL", configURL,
			"clientID", auth.GitHubApp.ClientID,
			"installationID", auth.GitHubApp.InstallationID,
		)
		return scaleset.NewClientWithGitHubApp(scaleset.ClientWithGitHubAppConfig{
			GitHubConfigURL: configURL,
			GitHubAppAuth: scaleset.GitHubAppAuth{
				ClientID:       auth.GitHubApp.ClientID,
				InstallationID: auth.GitHubApp.InstallationID,
				PrivateKey:     string(pemBytes),
			},
		})
	default:
		logger.Info("authenticating with PAT", "configURL", configURL)
		return scaleset.NewClientWithPersonalAccessToken(
			scaleset.NewClientWithPersonalAccessTokenConfig{
				GitHubConfigURL:     configURL,
				PersonalAccessToken: *auth.Token,
			},
		)
	}
}

func launchController(wg *sync.WaitGroup, ctx context.Context, rs *config.RunnerSetConfig, runnerGroup string, idleTimeout time.Duration, client *scaleset.Client, logger *slog.Logger) error {
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

	d := controller.New(rs, runnerGroup, idleTimeout, client, b, rsLogger)

	rsLogger.Info("launching controller",
		"runnerGroup", runnerGroup,
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

	return nil
}
