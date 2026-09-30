package main

import (
	"context"
	"fmt"
	"os"

	"github.com/boring-design/elastic-fruit-runner/config"
	"github.com/boring-design/elastic-fruit-runner/internal/auth"
)

func resetAdminPassword(configPath string) error {
	cfg, err := config.Load(configPath)
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
	fmt.Fprintln(os.Stdout, "Admin password cleared. Restart the service, then open the console to create a new admin password.")
	return nil
}
