package config

import (
	"bufio"
	"flag"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config holds all runtime configuration for the daemon.
type Config struct {
	GitHub      GitHubConfig      `yaml:"github"`
	RunnerGroup string            `yaml:"runner_group"`
	IdleTimeout time.Duration     `yaml:"idle_timeout"`
	RunnerSets  []RunnerSetConfig `yaml:"runner_sets"`
}

// GitHubConfig holds GitHub authentication and connection settings.
type GitHubConfig struct {
	URL   string          `yaml:"url"`
	Token string          `yaml:"token"`
	App   GitHubAppConfig `yaml:"app"`
}

// GitHubAppConfig holds GitHub App authentication settings.
type GitHubAppConfig struct {
	ClientID       string `yaml:"client_id"`
	InstallationID int64  `yaml:"installation_id"`
	PrivateKeyPath string `yaml:"private_key_path"`
}

// RunnerSetConfig describes a single runner scale set.
type RunnerSetConfig struct {
	Name       string   `yaml:"name"`
	Backend    string   `yaml:"backend"`
	Image      string   `yaml:"image"`
	Labels     []string `yaml:"labels"`
	MaxRunners int      `yaml:"max_runners"`
	Platform   string   `yaml:"platform"`
}

// AuthMode returns which authentication method is configured.
func (c *Config) AuthMode() string {
	if c.GitHub.App.ClientID != "" {
		return "app"
	}
	return "pat"
}

// Validate returns an error if the configuration is invalid.
func (c *Config) Validate() error {
	if c.GitHub.URL == "" {
		return fmt.Errorf("github.url is required (config file, --url flag, or GITHUB_CONFIG_URL env)")
	}
	if _, err := url.ParseRequestURI(c.GitHub.URL); err != nil {
		return fmt.Errorf("invalid github.url %q: %w", c.GitHub.URL, err)
	}

	switch c.AuthMode() {
	case "app":
		if c.GitHub.App.InstallationID == 0 {
			return fmt.Errorf("github.app.installation_id is required for GitHub App auth")
		}
		if c.GitHub.App.PrivateKeyPath == "" {
			return fmt.Errorf("github.app.private_key_path is required for GitHub App auth")
		}
	default:
		if c.GitHub.Token == "" {
			return fmt.Errorf("no auth configured: set github.token (PAT) or github.app (GitHub App)")
		}
	}

	if c.IdleTimeout <= 0 {
		return fmt.Errorf("idle_timeout must be greater than 0")
	}

	if len(c.RunnerSets) == 0 {
		return fmt.Errorf("at least one runner_sets entry is required")
	}
	for i, rs := range c.RunnerSets {
		if rs.Name == "" {
			return fmt.Errorf("runner_sets[%d].name is required", i)
		}
		if rs.Backend == "" {
			return fmt.Errorf("runner_sets[%d].backend is required", i)
		}
		if rs.Backend != "tart" && rs.Backend != "docker" && rs.Backend != "host" {
			return fmt.Errorf("runner_sets[%d].backend must be 'tart', 'docker', or 'host', got %q", i, rs.Backend)
		}
		if rs.MaxRunners <= 0 {
			return fmt.Errorf("runner_sets[%d].max_runners must be > 0", i)
		}
	}

	return nil
}

// defaults returns a Config with sensible default values.
// runner_sets is intentionally empty — users must configure it explicitly.
func defaults() *Config {
	return &Config{
		RunnerGroup: "Default",
		IdleTimeout: 15 * time.Minute,
	}
}

// redact replaces a secret string with a masked version showing only the
// last 4 characters, or "***" if shorter than 5 characters.
func redact(s string) string {
	if s == "" {
		return "(not set)"
	}
	if len(s) <= 4 {
		return "***"
	}
	return "***" + s[len(s)-4:]
}

// RedactedSlogAttrs returns slog attributes representing the full config
// with secrets masked. Suitable for logging at startup.
func (c *Config) RedactedSlogAttrs() []any {
	attrs := []any{
		"github.url", c.GitHub.URL,
		"auth_mode", c.AuthMode(),
		"runner_group", c.RunnerGroup,
		"idle_timeout", c.IdleTimeout.String(),
	}

	switch c.AuthMode() {
	case "app":
		attrs = append(attrs,
			"github.app.client_id", c.GitHub.App.ClientID,
			"github.app.installation_id", c.GitHub.App.InstallationID,
			"github.app.private_key_path", c.GitHub.App.PrivateKeyPath,
		)
	default:
		attrs = append(attrs, "github.token", redact(c.GitHub.Token))
	}

	for i, rs := range c.RunnerSets {
		prefix := fmt.Sprintf("runner_sets[%d].", i)
		attrs = append(attrs,
			prefix+"name", rs.Name,
			prefix+"backend", rs.Backend,
			prefix+"image", rs.Image,
			prefix+"labels", rs.Labels,
			prefix+"max_runners", rs.MaxRunners,
		)
		if rs.Platform != "" {
			attrs = append(attrs, prefix+"platform", rs.Platform)
		}
	}

	return attrs
}

// configFilePaths returns candidate config file paths in priority order.
func configFilePaths() []string {
	var paths []string
	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths, filepath.Join(home, ".elastic-fruit-runner", "config.yaml"))
	}
	paths = append(paths,
		"/opt/homebrew/var/elastic-fruit-runner/config.yaml",
		"/usr/local/var/elastic-fruit-runner/config.yaml",
		"/etc/elastic-fruit-runner/config.yaml",
	)
	return paths
}

// envFilePaths returns candidate env file paths in priority order.
func envFilePaths() []string {
	var paths []string
	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths, filepath.Join(home, ".elastic-fruit-runner", "env"))
	}
	paths = append(paths,
		"/opt/homebrew/var/elastic-fruit-runner/env",
		"/usr/local/var/elastic-fruit-runner/env",
	)
	return paths
}

// loadEnvFile reads KEY=VALUE lines from the given path and sets them as
// environment variables. Existing env vars are NOT overwritten, so real
// environment always takes precedence over the file.
func loadEnvFile(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		if _, exists := os.LookupEnv(k); !exists {
			os.Setenv(k, v)
		}
	}
}

// loadConfigFile reads and parses a YAML config file into cfg.
// Returns true if a file was found and loaded.
func loadConfigFile(path string, cfg *Config) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("read config %s: %w", path, err)
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return false, fmt.Errorf("parse config %s: %w", path, err)
	}
	return true, nil
}

// Load reads configuration from (in order of ascending priority):
//  1. Built-in defaults
//  2. Config file (YAML)
//  3. Env file (~/.elastic-fruit-runner/env)
//  4. Environment variables
//  5. CLI flags (--config, --url, --token)
func Load() (*Config, error) {
	cfg := defaults()

	var configPath string
	flag.StringVar(&configPath, "config", "", "Path to config file (default: ~/.elastic-fruit-runner/config.yaml)")

	// Keep a few essential flags for quick CLI usage
	var flagURL, flagToken string
	flag.StringVar(&flagURL, "url", "", "GitHub config URL (overrides config file)")
	flag.StringVar(&flagToken, "token", "", "GitHub PAT (overrides config file)")
	flag.Parse()

	// Load config file
	if configPath != "" {
		if _, err := loadConfigFile(configPath, cfg); err != nil {
			return nil, err
		}
	} else {
		for _, p := range configFilePaths() {
			found, err := loadConfigFile(p, cfg)
			if err != nil {
				return nil, err
			}
			if found {
				break
			}
		}
	}

	// Load env file (sets env vars that aren't already set)
	for _, p := range envFilePaths() {
		loadEnvFile(p)
	}

	// Env var overrides (config file < env var)
	if v := os.Getenv("GITHUB_CONFIG_URL"); v != "" {
		cfg.GitHub.URL = v
	}
	if v := os.Getenv("GITHUB_TOKEN"); v != "" {
		cfg.GitHub.Token = v
	}
	if v := os.Getenv("GITHUB_APP_CLIENT_ID"); v != "" {
		cfg.GitHub.App.ClientID = v
	}
	if v := os.Getenv("GITHUB_APP_INSTALLATION_ID"); v != "" {
		fmt.Sscanf(v, "%d", &cfg.GitHub.App.InstallationID)
	}
	if v := os.Getenv("GITHUB_APP_PRIVATE_KEY_PATH"); v != "" {
		cfg.GitHub.App.PrivateKeyPath = v
	}
	if v := os.Getenv("GITHUB_RUNNER_GROUP"); v != "" {
		cfg.RunnerGroup = v
	}

	// Flag overrides (highest priority)
	if flagURL != "" {
		cfg.GitHub.URL = flagURL
	}
	if flagToken != "" {
		cfg.GitHub.Token = flagToken
	}

	return cfg, nil
}
