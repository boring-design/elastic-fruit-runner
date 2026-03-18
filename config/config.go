package config

import (
	"flag"
	"fmt"
	"net/url"
	"os"
	"time"
)

// Config holds all runtime configuration for the daemon.
type Config struct {
	// PAT auth
	GitHubToken string
	// GitHub App auth
	AppClientID       string
	AppInstallationID int64
	AppPrivateKeyPath string
	// Common
	GitHubURL   string
	RunnerGroup string
	VMImage     string
	IdleTimeout time.Duration
}

// RunnerSetConfig describes a single runner scale set.
type RunnerSetConfig struct {
	Name       string
	Backend    string // "tart" or "docker"
	Image      string
	Labels     []string
	MaxRunners int
	Platform   string // docker-only, e.g. "linux/amd64"
}

// DefaultRunnerSets returns the 3 hardcoded runner scale set configurations.
func DefaultRunnerSets(cfg *Config) []RunnerSetConfig {
	return []RunnerSetConfig{
		{
			Name:       "efr-macos-arm64",
			Backend:    "tart",
			Image:      cfg.VMImage,
			Labels:     []string{"self-hosted", "macOS", "ARM64"},
			MaxRunners: 2,
		},
		{
			Name:       "efr-linux-arm64",
			Backend:    "docker",
			Image:      "ghcr.io/actions/actions-runner:latest",
			Labels:     []string{"self-hosted", "Linux", "ARM64"},
			MaxRunners: 4,
		},
		{
			Name:       "efr-linux-amd64",
			Backend:    "docker",
			Image:      "ghcr.io/actions/actions-runner:latest",
			Labels:     []string{"self-hosted", "Linux", "X64"},
			MaxRunners: 4,
			Platform:   "linux/amd64",
		},
	}
}

// AuthMode returns which authentication method is configured.
func (c *Config) AuthMode() string {
	if c.AppClientID != "" {
		return "app"
	}
	return "pat"
}

// Validate returns an error if the configuration is invalid.
func (c *Config) Validate() error {
	if c.GitHubURL == "" {
		return fmt.Errorf("GitHub config URL is required: set --url or GITHUB_CONFIG_URL")
	}
	if _, err := url.ParseRequestURI(c.GitHubURL); err != nil {
		return fmt.Errorf("invalid GitHub config URL %q: %w", c.GitHubURL, err)
	}

	switch c.AuthMode() {
	case "app":
		if c.AppInstallationID == 0 {
			return fmt.Errorf("GitHub App auth requires --app-installation-id or GITHUB_APP_INSTALLATION_ID")
		}
		if c.AppPrivateKeyPath == "" {
			return fmt.Errorf("GitHub App auth requires --app-private-key or GITHUB_APP_PRIVATE_KEY_PATH")
		}
	default:
		if c.GitHubToken == "" {
			return fmt.Errorf("no auth configured: set --token (PAT) or --app-client-id (GitHub App)")
		}
	}

	if c.IdleTimeout <= 0 {
		return fmt.Errorf("--idle-timeout must be greater than 0")
	}
	return nil
}

// Load parses flags and environment variables into a Config.
// Flags take precedence over environment variables.
func Load() *Config {
	cfg := &Config{}

	// PAT auth
	flag.StringVar(&cfg.GitHubToken, "token", os.Getenv("GITHUB_TOKEN"),
		"GitHub PAT (or GITHUB_TOKEN env). Used when --app-client-id is not set.")

	// GitHub App auth
	flag.StringVar(&cfg.AppClientID, "app-client-id", os.Getenv("GITHUB_APP_CLIENT_ID"),
		"GitHub App Client ID (or GITHUB_APP_CLIENT_ID env)")
	var installationID int64
	flag.Int64Var(&installationID, "app-installation-id", 0,
		"GitHub App Installation ID (or GITHUB_APP_INSTALLATION_ID env)")
	flag.StringVar(&cfg.AppPrivateKeyPath, "app-private-key", os.Getenv("GITHUB_APP_PRIVATE_KEY_PATH"),
		"Path to GitHub App private key PEM file (or GITHUB_APP_PRIVATE_KEY_PATH env)")

	// Common
	flag.StringVar(&cfg.GitHubURL, "url", os.Getenv("GITHUB_CONFIG_URL"),
		"GitHub config URL — org or repo, e.g. https://github.com/myorg (or GITHUB_CONFIG_URL env)")
	flag.StringVar(&cfg.RunnerGroup, "runner-group", envOrDefault("GITHUB_RUNNER_GROUP", "Default"),
		"Runner group name")
	flag.StringVar(&cfg.VMImage, "vm-image", envOrDefault("TART_VM_IMAGE", "ghcr.io/cirruslabs/macos-sequoia-base:latest"),
		"Tart VM image to clone for each runner")
	flag.DurationVar(&cfg.IdleTimeout, "idle-timeout", 15*time.Minute,
		"How long an idle runner waits for a job before being shut down (or RUNNER_IDLE_TIMEOUT env)")

	flag.Parse()

	// Resolve installation ID: flag > env
	if installationID != 0 {
		cfg.AppInstallationID = installationID
	} else if v := os.Getenv("GITHUB_APP_INSTALLATION_ID"); v != "" {
		fmt.Sscanf(v, "%d", &cfg.AppInstallationID)
	}

	return cfg
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
