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
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))
	if err := run(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	logLevel, err := cfg.ParsedLogLevel()
	if err != nil {
		slog.Error("invalid log level", "configured", cfg.LogLevel, "valid_values", "debug, info, warn, error", "err", err)
		return fmt.Errorf("invalid log level %q: %w", cfg.LogLevel, err)
	}

	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	})))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	tracingShutdown, err := tracing.Setup(ctx)
	if err != nil {
		return fmt.Errorf("initialize tracing: %w", err)
	}
	defer func() {
		if shutdownErr := tracingShutdown(context.Background()); shutdownErr != nil {
			slog.Warn("tracing shutdown error", "err", shutdownErr)
		}
	}()

	var wg sync.WaitGroup

	for i := range cfg.Orgs {
		org := &cfg.Orgs[i]
		client, clientErr := createClient(org.ConfigURL(), &org.Auth)
		if clientErr != nil {
			return fmt.Errorf("create client for org %s: %w", org.Org, clientErr)
		}

		for j := range org.RunnerSets {
			rs := &org.RunnerSets[j]
			if err := launchController(ctx, &wg, rs, org.RunnerGroup, cfg.IdleTimeout, client); err != nil {
				return fmt.Errorf("launch controller for runner set %s: %w", rs.Name, err)
			}
		}
	}

	for i := range cfg.Repos {
		repo := &cfg.Repos[i]
		client, clientErr := createClient(repo.ConfigURL(), &repo.Auth)
		if clientErr != nil {
			return fmt.Errorf("create client for repo %s: %w", repo.Repo, clientErr)
		}

		for j := range repo.RunnerSets {
			rs := &repo.RunnerSets[j]
			if err := launchController(ctx, &wg, rs, "Default", cfg.IdleTimeout, client); err != nil {
				return fmt.Errorf("launch controller for runner set %s: %w", rs.Name, err)
			}
		}
	}

	wg.Wait()
	slog.Info("shutdown complete")
	return nil
}

func createClient(configURL string, auth *config.AuthConfig) (*scaleset.Client, error) {
	switch auth.Mode() {
	case config.AuthModeGitHubApp:
		pemBytes, readErr := os.ReadFile(auth.GitHubApp.PrivateKeyPath)
		if readErr != nil {
			return nil, fmt.Errorf("read GitHub App private key %s: %w", auth.GitHubApp.PrivateKeyPath, readErr)
		}
		slog.Info("authenticating with GitHub App",
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
	case config.AuthModePAT:
		slog.Info("authenticating with PAT", "configURL", configURL)
		return scaleset.NewClientWithPersonalAccessToken(
			scaleset.NewClientWithPersonalAccessTokenConfig{
				GitHubConfigURL:     configURL,
				PersonalAccessToken: *auth.PATToken,
			},
		)
	default:
		return nil, fmt.Errorf("unknown auth mode %q", auth.Mode())
	}
}

func launchController(ctx context.Context, wg *sync.WaitGroup, rs *config.RunnerSetConfig, runnerGroup string, idleTimeout time.Duration, client *scaleset.Client) error {
	var b backend.Backend
	switch rs.Backend {
	case "tart":
		b = backend.NewTartBackend(rs.Image)
	case "docker":
		b = backend.NewDockerBackend(rs.Image, rs.Platform)
	default:
		return fmt.Errorf("unknown backend %q for runner set %q", rs.Backend, rs.Name)
	}

	d := controller.New(rs, runnerGroup, idleTimeout, client, b)

	slog.Info("launching controller",
		"runnerSet", rs.Name,
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
				slog.Info("controller stopped", "runnerSet", rs.Name, "err", err)
				return
			}
			slog.Error("controller exited with error, restarting", "runnerSet", rs.Name, "err", err)
			time.Sleep(5 * time.Second)
		}
	}()

	return nil
}
