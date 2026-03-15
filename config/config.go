package config

import (
	"flag"
	"fmt"
	"os"
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
	GitHubURL    string
	RunnerGroup  string
	ScaleSetName string
	VMImage      string
	MaxRunners int
	PoolSize   int
	// Backend mode: "host" or "tart"
	Mode string
}

// AuthMode returns which authentication method is configured.
func (c *Config) AuthMode() string {
	if c.AppClientID != "" {
		return "app"
	}
	return "pat"
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
	flag.StringVar(&cfg.ScaleSetName, "scale-set-name", envOrDefault("SCALE_SET_NAME", "elastic-fruit-runner"),
		"Scale set name shown in GitHub Actions")
	flag.StringVar(&cfg.VMImage, "vm-image", envOrDefault("TART_VM_IMAGE", "ghcr.io/cirruslabs/macos-sequoia-base:latest"),
		"Tart VM image to clone for each runner")
	flag.IntVar(&cfg.MaxRunners, "max-runners", 2,
		"Maximum concurrent runners (Apple EULA limit for macOS VMs is 2)")
	flag.IntVar(&cfg.PoolSize, "pool-size", 0,
		"Number of pre-warmed runner slots (defaults to --max-runners if 0)")
	flag.StringVar(&cfg.Mode, "mode", envOrDefault("RUNNER_MODE", "tart"),
		"Backend mode: 'host' (run on host directly) or 'tart' (ephemeral VMs)")

	flag.Parse()

	// Resolve installation ID: flag > env
	if installationID != 0 {
		cfg.AppInstallationID = installationID
	} else if v := os.Getenv("GITHUB_APP_INSTALLATION_ID"); v != "" {
		fmt.Sscanf(v, "%d", &cfg.AppInstallationID)
	}

	// Default pool size to max runners.
	if cfg.PoolSize == 0 {
		cfg.PoolSize = cfg.MaxRunners
	}

	return cfg
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
