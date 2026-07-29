package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
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
	if len(os.Args) > 1 && os.Args[1] == "reset-password" {
		return resetAdminPassword(os.Args[2:])
	}
	return runDaemon()
}

func runDaemon() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	if err := configureLogging(cfg); err != nil {
		return err
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

	managementService, err := management.New(cfg)
	if err != nil {
		return fmt.Errorf("initialize scale set controller management service: %w", err)
	}
	defer managementService.Close()
	vitalsService.SetOnUpdate(managementService.RecordHostVitals)
	go vitalsService.Start(ctx, 5*time.Second)
	managementService.Start(ctx)

	databasePath, err := cfg.DatabasePath()
	if err != nil {
		return fmt.Errorf("resolve console database path: %w", err)
	}
	authService, err := auth.Open(databasePath)
	if err != nil {
		return fmt.Errorf("initialize console auth: %w", err)
	}
	defer authService.Close()
	logSetupCode(authService)

	configStateService := configstate.New(cfg, startedAt)
	go configStateService.Start(ctx, 2*time.Second)

	apiAddr := cfg.APIAddr
	if apiAddr == "" {
		apiAddr = ":8080"
	}
	apiServer := api.NewServer(
		managementService,
		vitalsService,
		cfg.IdleTimeout,
		cfg.CORS,
		api.Dependencies{
			Auth:         authService,
			ConfigState:  configStateService,
			DatabasePath: databasePath,
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
	go func() {
		managementService.Wait()
		close(done)
	}()

	select {
	case err := <-listenErr:
		return fmt.Errorf("API server failed to start: %w", err)
	case <-done:
		slog.Info("shutdown complete")
		return nil
	}
}

func logSetupCode(authService *auth.Service) {
	if setupCode := authService.SetupCode(); setupCode != "" {
		slog.Warn("console admin setup required", "setup_code", setupCode)
	}
}

func resetAdminPassword(args []string) error {
	cfg, err := config.LoadWithArgs(args)
	if err != nil {
		return fmt.Errorf("load configuration for password reset: %w", err)
	}
	databasePath, err := cfg.DatabasePath()
	if err != nil {
		return fmt.Errorf("resolve console database path for password reset: %w", err)
	}
	authService, err := auth.Open(databasePath)
	if err != nil {
		return fmt.Errorf("open console auth for password reset: %w", err)
	}
	defer authService.Close()
	if err := authService.Reset(context.Background()); err != nil {
		return fmt.Errorf("reset console admin password in %s: %w", databasePath, err)
	}
	fmt.Fprintln(os.Stdout, "Admin password cleared. Restart the service to get a new setup code.")
	return nil
}

func configureLogging(cfg *config.Config) error {
	logLevel, err := cfg.ParsedLogLevel()
	if err != nil {
		slog.Error("invalid log level", "configured", cfg.LogLevel, "valid_values", "debug, info, warn, error", "err", err)
		return fmt.Errorf("invalid log level %q: %w", cfg.LogLevel, err)
	}

	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	})))
	return nil
}
