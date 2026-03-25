package config

import (
	"fmt"
	"net/url"
	"strings"
	"time"
)

// Config holds all runtime configuration for the daemon.
type Config struct {
	Orgs        []OrgConfig   `yaml:"orgs"`
	Repos       []RepoConfig  `yaml:"repos"`
	IdleTimeout time.Duration `yaml:"idle_timeout"`
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

// AuthConfig holds authentication credentials for a GitHub org or repo.
// Exactly one of GitHubApp or Token must be set.
type AuthConfig struct {
	GitHubApp *GitHubAppConfig `yaml:"github_app"`
	Token     *string          `yaml:"token"`
}

// Mode returns which authentication method is configured: "app" or "pat".
func (a *AuthConfig) Mode() string {
	if a.GitHubApp != nil && a.GitHubApp.ClientID != "" {
		return "app"
	}
	return "pat"
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

	runnerSetNames := make(map[string]struct{})

	for i := range c.Orgs {
		org := &c.Orgs[i]
		if org.Org == "" {
			return fmt.Errorf("orgs[%d].org is required", i)
		}
		if _, err := url.ParseRequestURI(org.ConfigURL()); err != nil {
			return fmt.Errorf("orgs[%d]: invalid org %q: %w", i, org.Org, err)
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
	hasToken := auth.Token != nil
	hasApp := auth.GitHubApp != nil

	if !hasToken && !hasApp {
		return fmt.Errorf("%s.auth: one of token or github_app must be configured", prefix)
	}
	if hasToken && hasApp {
		return fmt.Errorf("%s.auth: token and github_app are mutually exclusive", prefix)
	}

	if hasToken && *auth.Token == "" {
		return fmt.Errorf("%s.auth.token must not be empty", prefix)
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
		"idle_timeout", c.IdleTimeout.String(),
	}

	for i, org := range c.Orgs {
		prefix := fmt.Sprintf("orgs[%d].", i)
		attrs = append(attrs,
			prefix+"org", org.Org,
			prefix+"auth_mode", org.Auth.Mode(),
			prefix+"runner_group", org.RunnerGroup,
		)
		switch org.Auth.Mode() {
		case "app":
			attrs = append(attrs,
				prefix+"auth.github_app.client_id", org.Auth.GitHubApp.ClientID,
				prefix+"auth.github_app.installation_id", org.Auth.GitHubApp.InstallationID,
				prefix+"auth.github_app.private_key_path", org.Auth.GitHubApp.PrivateKeyPath,
			)
		default:
			tok := ""
			if org.Auth.Token != nil {
				tok = *org.Auth.Token
			}
			attrs = append(attrs, prefix+"auth.token", redact(tok))
		}
		for j, rs := range org.RunnerSets {
			rsPrefix := fmt.Sprintf("%srunner_sets[%d].", prefix, j)
			attrs = appendRunnerSetAttrs(attrs, rsPrefix, &rs)
		}
	}

	for i, repo := range c.Repos {
		prefix := fmt.Sprintf("repos[%d].", i)
		attrs = append(attrs,
			prefix+"repo", repo.Repo,
			prefix+"auth_mode", repo.Auth.Mode(),
		)
		switch repo.Auth.Mode() {
		case "app":
			attrs = append(attrs,
				prefix+"auth.github_app.client_id", repo.Auth.GitHubApp.ClientID,
				prefix+"auth.github_app.installation_id", repo.Auth.GitHubApp.InstallationID,
				prefix+"auth.github_app.private_key_path", repo.Auth.GitHubApp.PrivateKeyPath,
			)
		default:
			tok := ""
			if repo.Auth.Token != nil {
				tok = *repo.Auth.Token
			}
			attrs = append(attrs, prefix+"auth.token", redact(tok))
		}
		for j, rs := range repo.RunnerSets {
			rsPrefix := fmt.Sprintf("%srunner_sets[%d].", prefix, j)
			attrs = appendRunnerSetAttrs(attrs, rsPrefix, &rs)
		}
	}

	return attrs
}

func appendRunnerSetAttrs(attrs []any, prefix string, rs *RunnerSetConfig) []any {
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
	return attrs
}
