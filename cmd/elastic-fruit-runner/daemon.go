package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/boring-design/elastic-fruit-runner/config"
	"github.com/boring-design/elastic-fruit-runner/internal/api"
	"github.com/boring-design/elastic-fruit-runner/internal/auth"
	"github.com/boring-design/elastic-fruit-runner/internal/configstate"
	"github.com/boring-design/elastic-fruit-runner/internal/management"
	"github.com/boring-design/elastic-fruit-runner/internal/tracing"
	"github.com/boring-design/elastic-fruit-runner/internal/vitals"
)

//nolint:gocyclo // Startup handles normal, recovery, and config mode in one ordered flow.
func runDaemon(requestedPath string) error {
	configPath := config.FindConfigPath(requestedPath)
	databasePath, err := config.DefaultDatabasePath()
	if err != nil {
		return err
	}
	revisionPath := databasePath
	cfg, configErr := config.Load(requestedPath)
	if configErr == nil {
		if err := cfg.Validate(); err != nil {
			configErr = err
		}
	}
	if cfg != nil {
		if path, pathErr := cfg.DatabasePath(); pathErr == nil {
			databasePath = path
		}
	}
	if configErr != nil {
		recovered, recoverErr := configstate.LoadLastActive(revisionPath)
		if recoverErr == nil {
			result := config.ValidateYAML(recovered)
			if len(result.Errors) == 0 {
				cfg = result.Config
				cfg.FilePath = configPath
				cfg.LoadedYAML = recovered
				slog.Warn("disk config is invalid, using last active config", "path", configPath, "err", configErr)
			} else {
				cfg = nil
			}
		} else {
			cfg = nil
		}
	}
	if cfg != nil {
		if err := configureLogging(cfg); err != nil {
			return err
		}
	} else {
		slog.Warn("starting in config mode", "path", configPath, "err", configErr)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	startedAt := time.Now()

	tracingShutdown, err := tracing.Setup(ctx)
	if err != nil {
		return fmt.Errorf("initialize tracing: %w", err)
	}
	defer func() {
		if shutdownErr := tracingShutdown(context.Background()); shutdownErr != nil {
			slog.Warn("tracing shutdown error", "err", shutdownErr)
		}
	}()

	vitalsService := vitals.New(startedAt)

	var managementService *management.Service
	if cfg != nil {
		managementService, err = management.New(cfg)
		if err != nil {
			return fmt.Errorf("initialize scale set controller management service: %w", err)
		}
		defer managementService.Close()
		vitalsService.SetOnUpdate(managementService.RecordHostVitals)
		managementService.Start(ctx)
	}
	go vitalsService.Start(ctx, 5*time.Second)

	authService, err := auth.Open(databasePath)
	if err != nil {
		return fmt.Errorf("initialize console auth: %w", err)
	}
	defer authService.Close()
	logSetupRequired(ctx, authService)

	var configStateService *configstate.Service
	if cfg != nil {
		configStateService = configstate.New(cfg, startedAt, revisionPath)
	} else {
		configStateService = configstate.NewForConfigMode(configPath, revisionPath, startedAt)
	}
	defer configStateService.Close()
	go configStateService.Start(ctx, 2*time.Second)

	apiAddr := ""
	cors := config.CORSConfig{}
	idleTimeout := 15 * time.Minute
	if cfg != nil {
		apiAddr = cfg.APIAddr
		cors = cfg.CORS
		idleTimeout = cfg.IdleTimeout
	}
	if apiAddr == "" {
		apiAddr = ":8080"
	}
	apiServer := api.NewServer(
		managementService,
		vitalsService,
		idleTimeout,
		cors,
		api.Dependencies{
			Auth:         authService,
			ConfigState:  configStateService,
			DatabasePath: databasePath,
			LogPath:      configLogPath(cfg),
		},
	)
	httpServer := &http.Server{
		Addr:              apiAddr,
		Handler:           apiServer.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	listenErr := make(chan error, 1)
	go func() {
		slog.Info("API server starting", "addr", apiAddr)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			listenErr <- err
		}
	}()

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			slog.Error("API server shutdown error", "err", err)
		}
	}()

	done := make(chan struct{})
	if managementService != nil {
		go func() {
			managementService.Wait()
			close(done)
		}()
	} else {
		go func() {
			<-ctx.Done()
			close(done)
		}()
	}

	select {
	case err := <-listenErr:
		return fmt.Errorf("API server failed to start: %w", err)
	case <-done:
		slog.Info("shutdown complete")
		return nil
	}
}

func logSetupRequired(ctx context.Context, authService *auth.Service) {
	setupRequired, err := authService.SetupRequired(ctx)
	if err != nil {
		slog.Warn("check console admin setup state", "err", err)
		return
	}
	if setupRequired {
		slog.Warn("console admin setup required, open the console to create the admin password")
	}
}

func configureLogging(cfg *config.Config) error {
	logLevel, err := cfg.ParsedLogLevel()
	if err != nil {
		slog.Error("invalid log level", "configured", cfg.LogLevel, "valid_values", "debug, info, warn, error", "err", err)
		return fmt.Errorf("invalid log level %q: %w", cfg.LogLevel, err)
	}

	output := io.Writer(os.Stdout)
	if cfg.LogPath != "" {
		if err := os.MkdirAll(filepath.Dir(cfg.LogPath), 0o750); err != nil {
			return fmt.Errorf("create log directory %s: %w", filepath.Dir(cfg.LogPath), err)
		}
		file, openErr := os.OpenFile(cfg.LogPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
		if openErr != nil {
			return fmt.Errorf("open log file %s: %w", cfg.LogPath, openErr)
		}
		if chmodErr := file.Chmod(0o600); chmodErr != nil {
			_ = file.Close()
			return fmt.Errorf("set log file permissions %s: %w", cfg.LogPath, chmodErr)
		}
		output = io.MultiWriter(os.Stdout, file)
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(output, &slog.HandlerOptions{
		Level: logLevel,
	})))
	return nil
}

func configLogPath(cfg *config.Config) string {
	if cfg == nil {
		return ""
	}
	return cfg.LogPath
}
