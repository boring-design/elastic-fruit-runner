package config

import (
	"flag"
	"os"
)

// Config holds all runtime configuration for the daemon.
type Config struct {
	GitHubToken  string
	GitHubURL    string
	RunnerGroup  string
	ScaleSetName string
	VMImage      string
	MaxRunners   int
}

// Load parses flags and environment variables into a Config.
// Flags take precedence over environment variables.
func Load() *Config {
	cfg := &Config{}

	flag.StringVar(&cfg.GitHubToken, "token", os.Getenv("GITHUB_TOKEN"),
		"GitHub personal access token (or GITHUB_TOKEN env var)")
	flag.StringVar(&cfg.GitHubURL, "url", os.Getenv("GITHUB_CONFIG_URL"),
		"GitHub config URL, e.g. https://github.com/myorg (or GITHUB_CONFIG_URL env var)")
	flag.StringVar(&cfg.RunnerGroup, "runner-group", envOrDefault("GITHUB_RUNNER_GROUP", "Default"),
		"Runner group name")
	flag.StringVar(&cfg.ScaleSetName, "scale-set-name", envOrDefault("SCALE_SET_NAME", "elastic-fruit-runner"),
		"Scale set name shown in GitHub Actions")
	flag.StringVar(&cfg.VMImage, "vm-image", envOrDefault("TART_VM_IMAGE", "ghcr.io/cirruslabs/macos-sequoia-base:latest"),
		"Tart VM image to clone for each runner")
	flag.IntVar(&cfg.MaxRunners, "max-runners", 2,
		"Maximum concurrent runners (Apple EULA limit for macOS VMs is 2)")

	flag.Parse()
	return cfg
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
