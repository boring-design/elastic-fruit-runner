package config

import (
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"
)

// Config holds all runtime configuration for the daemon.
type Config struct {
	GitHub      GitHubConfig      `yaml:"github"`
	RunnerGroup string            `yaml:"runner_group"`
	IdleTimeout time.Duration     `yaml:"idle_timeout"`
	LogLevel    string            `yaml:"log_level"`
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

	switch strings.ToLower(c.LogLevel) {
	case "debug", "info", "warn", "error", "":
	default:
		return fmt.Errorf("log_level %q is invalid; must be one of: debug, info, warn, error", c.LogLevel)
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

// ParsedLogLevel converts the LogLevel string to a slog.Level.
// Recognized values: debug, info, warn, error (case-insensitive).
// Empty string defaults to slog.LevelInfo. Unrecognized values return an error.
func (c *Config) ParsedLogLevel() (slog.Level, error) {
	switch strings.ToLower(c.LogLevel) {
	case "debug":
		return slog.LevelDebug, nil
	case "info", "":
		return slog.LevelInfo, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, fmt.Errorf("unrecognized log level %q", c.LogLevel)
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
		"log_level", c.LogLevel,
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

