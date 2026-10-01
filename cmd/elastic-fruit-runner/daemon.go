package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/boring-design/elastic-fruit-runner/config"
	"github.com/boring-design/elastic-fruit-runner/internal/api"
	"github.com/boring-design/elastic-fruit-runner/internal/auth"
	"github.com/boring-design/elastic-fruit-runner/internal/configstate"
	"github.com/boring-design/elastic-fruit-runner/internal/management"
	"github.com/boring-design/elastic-fruit-runner/internal/storage"
	"github.com/boring-design/elastic-fruit-runner/internal/tracing"
	"github.com/boring-design/elastic-fruit-runner/internal/vitals"
)

// errRestartRequested tells main to re-exec the daemon after a clean shutdown.
var errRestartRequested = errors.New("restart requested from console")

// startupState holds what the daemon learned before any service starts.
type startupState struct {
	configPath   string
	databasePath string
	// cfg is nil when the daemon runs in config mode.
	cfg *config.Config
	db  *sql.DB
}

func runDaemon(requestedConfigPath string) error {
	startup, err := prepareStartup(requestedConfigPath)
	if err != nil {
		return err
	}
	defer startup.db.Close()
	return runServices(startup)
}

// prepareStartup loads the config, opens the database, and falls back to the
// last active config when the disk config is broken.
func prepareStartup(requestedConfigPath string) (*startupState, error) {
	configPath := config.FindConfigPath(requestedConfigPath)
	cfg, configErr := config.Load(requestedConfigPath)
	if configErr == nil {
		if err := cfg.Validate(); err != nil {
			configErr = err
		}
	}

	var databasePath string
	var pathErr error
	if cfg != nil {
		databasePath, pathErr = cfg.DatabasePath()
	} else {
		databasePath, pathErr = config.DefaultDatabasePath()
	}
	if pathErr != nil {
		return nil, pathErr
	}
	db, err := storage.Open(databasePath)
	if err != nil {
		return nil, err
	}

	if configErr != nil {
		cfg = nil
		recovered, recoverErr := configstate.LoadLastActive(context.Background(), db)
		if recoverErr == nil {
			result := config.ValidateYAML(recovered)
			if len(result.Errors) == 0 {
				cfg = result.Config
				cfg.FilePath = configPath
				cfg.LoadedYAML = recovered
				slog.Warn("disk config is invalid, using last active config", "path", configPath, "err", configErr)
			}
		}
	}
	if cfg != nil {
		if err := configureLogging(cfg); err != nil {
			db.Close()
			return nil, err
		}
	} else {
		slog.Warn("starting in config mode", "path", configPath, "err", configErr)
	}
	slog.Info("database ready", "path", databasePath)

	return &startupState{
		configPath:   configPath,
		databasePath: databasePath,
		cfg:          cfg,
		db:           db,
	}, nil
}

//nolint:gocyclo // Startup handles normal, recovery, and config mode in one ordered flow.
func runServices(startup *startupState) error {
	cfg := startup.cfg
	configPath := startup.configPath
	databasePath := startup.databasePath
	db := startup.db

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	startedAt := time.Now()
	var restartRequested atomic.Bool

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
		managementService, err = management.New(cfg, db)
		if err != nil {
			return fmt.Errorf("initialize scale set controller management service: %w", err)
		}
		vitalsService.SetOnUpdate(managementService.RecordHostVitals)
		managementService.Start(ctx)
	}
	go vitalsService.Start(ctx, 5*time.Second)

	authService := auth.New(db)
	logSetupRequired(ctx, authService)

	var configStateService *configstate.Service
	if cfg != nil {
		configStateService = configstate.New(cfg, startedAt, db)
	} else {
		configStateService = configstate.NewForConfigMode(configPath, db, startedAt)
	}
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
			ConfigMode:   cfg == nil,
			ActiveConfig: cfg,
			RequestRestart: func() {
				restartRequested.Store(true)
				stop()
			},
		},
	)
	go apiServer.RefreshProbes(ctx)
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

	httpDone := make(chan struct{})
	go func() {
		defer close(httpDone)
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
		// Wait for the listening socket to close so a re-exec can bind the same address.
		<-httpDone
		slog.Info("shutdown complete")
		if restartRequested.Load() {
			return errRestartRequested
		}
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
