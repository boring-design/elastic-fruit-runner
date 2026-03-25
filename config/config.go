package config

import (
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// Config holds all runtime configuration for the daemon.
type Config struct {
	Orgs        []OrgConfig   `yaml:"orgs"`
	Repos       []RepoConfig  `yaml:"repos"`
	IdleTimeout time.Duration `yaml:"idle_timeout"`
	LogLevel    string        `yaml:"log_level"`
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

// OrgConfig describes a GitHub organization to listen for jobs on.
type OrgConfig struct {
	Org         string            `yaml:"org"`
	Auth        AuthConfig        `yaml:"auth"`
	RunnerGroup string            `yaml:"runner_group"`
	RunnerSets  []RunnerSetConfig `yaml:"runner_sets"`
}

// ConfigURL returns the GitHub Actions config URL for this org.
func (o *OrgConfig) ConfigURL() string {
	return "https://github.com/" + o.Org
}

// RepoConfig describes a GitHub repository to listen for jobs on.
type RepoConfig struct {
	Repo       string            `yaml:"repo"`
	Auth       AuthConfig        `yaml:"auth"`
	RunnerSets []RunnerSetConfig `yaml:"runner_sets"`
}

// ConfigURL returns the GitHub Actions config URL for this repo.
func (r *RepoConfig) ConfigURL() string {
	return "https://github.com/" + r.Repo
}

// AuthMode represents the authentication method.
type AuthMode string

const (
	AuthModeGitHubApp AuthMode = "app"
	AuthModePAT       AuthMode = "pat"
)

// AuthConfig holds authentication credentials for a GitHub org or repo.
// Exactly one of GitHubApp or PATToken must be set.
type AuthConfig struct {
	GitHubApp *GitHubAppConfig `yaml:"github_app"`
	PATToken  *string          `yaml:"pat_token"`
}

// Mode returns which authentication method is configured.
func (a *AuthConfig) Mode() AuthMode {
	if a.GitHubApp != nil && a.GitHubApp.ClientID != "" {
		return AuthModeGitHubApp
	}
	return AuthModePAT
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

// Validate returns an error if the configuration is invalid.
// It also applies defaults (e.g. runner_group → "Default").
func (c *Config) Validate() error {
	if len(c.Orgs) == 0 && len(c.Repos) == 0 {
		return fmt.Errorf("at least one org or repo must be configured")
	}

	if c.IdleTimeout <= 0 {
		return fmt.Errorf("idle_timeout must be greater than 0")
	}

	switch strings.ToLower(c.LogLevel) {
	case "debug", "info", "warn", "error", "":
	default:
		return fmt.Errorf("log_level %q is invalid; must be one of: debug, info, warn, error", c.LogLevel)
	}

	runnerSetNames := make(map[string]struct{})

	for i := range c.Orgs {
		org := &c.Orgs[i]
		if org.Org == "" {
			return fmt.Errorf("orgs[%d].org is required", i)
		}
		if err := validateAuth(&org.Auth, fmt.Sprintf("orgs[%d]", i)); err != nil {
			return err
		}
		if org.RunnerGroup == "" {
			org.RunnerGroup = "Default"
		}
		if len(org.RunnerSets) == 0 {
			return fmt.Errorf("orgs[%d].runner_sets must have at least one entry", i)
		}
		for j := range org.RunnerSets {
			if err := validateRunnerSet(&org.RunnerSets[j], fmt.Sprintf("orgs[%d].runner_sets[%d]", i, j), runnerSetNames); err != nil {
				return err
			}
		}
	}

	for i := range c.Repos {
		repo := &c.Repos[i]
		if repo.Repo == "" {
			return fmt.Errorf("repos[%d].repo is required", i)
		}
		parts := strings.SplitN(repo.Repo, "/", 3)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return fmt.Errorf("repos[%d].repo must be in \"owner/repo\" format, got %q", i, repo.Repo)
		}
		if err := validateAuth(&repo.Auth, fmt.Sprintf("repos[%d]", i)); err != nil {
			return err
		}
		if len(repo.RunnerSets) == 0 {
			return fmt.Errorf("repos[%d].runner_sets must have at least one entry", i)
		}
		for j := range repo.RunnerSets {
			if err := validateRunnerSet(&repo.RunnerSets[j], fmt.Sprintf("repos[%d].runner_sets[%d]", i, j), runnerSetNames); err != nil {
				return err
			}
		}
	}

	return nil
}

func validateAuth(auth *AuthConfig, prefix string) error {
	hasToken := auth.PATToken != nil
	hasApp := auth.GitHubApp != nil

	if !hasToken && !hasApp {
		return fmt.Errorf("%s.auth: one of pat_token or github_app must be configured", prefix)
	}
	if hasToken && hasApp {
		return fmt.Errorf("%s.auth: pat_token and github_app are mutually exclusive", prefix)
	}

	if hasToken && *auth.PATToken == "" {
		return fmt.Errorf("%s.auth.pat_token must not be empty", prefix)
	}

	if hasApp {
		app := auth.GitHubApp
		if app.ClientID == "" {
			return fmt.Errorf("%s.auth.github_app.client_id is required", prefix)
		}
		if app.InstallationID == 0 {
			return fmt.Errorf("%s.auth.github_app.installation_id is required", prefix)
		}
		if app.PrivateKeyPath == "" {
			return fmt.Errorf("%s.auth.github_app.private_key_path is required", prefix)
		}
	}

	return nil
}

func validateRunnerSet(rs *RunnerSetConfig, prefix string, seen map[string]struct{}) error {
	if rs.Name == "" {
		return fmt.Errorf("%s.name is required", prefix)
	}
	if _, exists := seen[rs.Name]; exists {
		return fmt.Errorf("runner set name %q is not unique", rs.Name)
	}
	seen[rs.Name] = struct{}{}

	if rs.Backend == "" {
		return fmt.Errorf("%s.backend is required", prefix)
	}
	if rs.Backend != "tart" && rs.Backend != "docker" && rs.Backend != "host" {
		return fmt.Errorf("%s.backend must be 'tart', 'docker', or 'host', got %q", prefix, rs.Backend)
	}
	if rs.MaxRunners <= 0 {
		return fmt.Errorf("%s.max_runners must be > 0", prefix)
	}

	return nil
}
