package config

import (
	"log/slog"
	"strings"
	"testing"
	"time"
)

func strPtr(s string) *string { return &s }

func validOrgConfig() OrgConfig {
	return OrgConfig{
		Org: "test-org",
		Auth: AuthConfig{
			PATToken: strPtr("ghp_test123"),
		},
		RunnerGroup: "Default",
		RunnerSets: []RunnerSetConfig{
			{
				Name:       "org-runner",
				Backend:    "docker",
				Image:      "test:latest",
				Labels:     []string{"self-hosted"},
				MaxRunners: 2,
			},
		},
	}
}

func validRepoConfig() RepoConfig {
	return RepoConfig{
		Repo: "owner/repo",
		Auth: AuthConfig{
			PATToken: strPtr("ghp_test456"),
		},
		RunnerSets: []RunnerSetConfig{
			{
				Name:       "repo-runner",
				Backend:    "docker",
				Image:      "test:latest",
				Labels:     []string{"self-hosted"},
				MaxRunners: 2,
			},
		},
	}
}

func validAppAuth() AuthConfig {
	return AuthConfig{
		GitHubApp: &GitHubAppConfig{
			ClientID:       "Iv23li_test",
			InstallationID: 12345678,
			PrivateKeyPath: "/path/to/key.pem",
		},
	}
}

func TestParsedLogLevel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input   string
		want    slog.Level
		wantErr bool
	}{
		{"debug", slog.LevelDebug, false},
		{"DEBUG", slog.LevelDebug, false},
		{"info", slog.LevelInfo, false},
		{"INFO", slog.LevelInfo, false},
		{"warn", slog.LevelWarn, false},
		{"warning", slog.LevelInfo, true},
		{"error", slog.LevelError, false},
		{"ERROR", slog.LevelError, false},
		{"", slog.LevelInfo, false},
		{"unknown", slog.LevelInfo, true},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			cfg := &Config{LogLevel: tt.input}
			got, err := cfg.ParsedLogLevel()
			if (err != nil) != tt.wantErr {
				t.Errorf("ParsedLogLevel(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("ParsedLogLevel(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestValidate_InvalidLogLevel(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Orgs:        []OrgConfig{validOrgConfig()},
		IdleTimeout: 15 * time.Minute,
		LogLevel:    "trace",
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected validation error for invalid log_level")
	}
	if !strings.Contains(err.Error(), "log_level") {
		t.Errorf("error should mention log_level, got: %v", err)
	}
}

func TestValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		cfg     Config
		wantErr string
	}{
		{
			name: "valid orgs only",
			cfg: Config{
				Orgs:        []OrgConfig{validOrgConfig()},
				IdleTimeout: 15 * time.Minute,
			},
		},
		{
			name: "valid repos only",
			cfg: Config{
				Repos:       []RepoConfig{validRepoConfig()},
				IdleTimeout: 15 * time.Minute,
			},
		},
		{
			name: "valid orgs and repos",
			cfg: Config{
				Orgs:        []OrgConfig{validOrgConfig()},
				Repos:       []RepoConfig{validRepoConfig()},
				IdleTimeout: 15 * time.Minute,
			},
		},
		{
			name: "no orgs and no repos",
			cfg: Config{
				IdleTimeout: 15 * time.Minute,
			},
			wantErr: "at least one org or repo must be configured",
		},
		{
			name: "org missing org field",
			cfg: Config{
				Orgs: []OrgConfig{func() OrgConfig {
					o := validOrgConfig()
					o.Org = ""
					return o
				}()},
				IdleTimeout: 15 * time.Minute,
			},
			wantErr: "orgs[0].org is required",
		},
		{
			name: "org with no auth",
			cfg: Config{
				Orgs: []OrgConfig{func() OrgConfig {
					o := validOrgConfig()
					o.Auth = AuthConfig{}
					return o
				}()},
				IdleTimeout: 15 * time.Minute,
			},
			wantErr: "orgs[0].auth: one of pat_token or github_app must be configured",
		},
		{
			name: "org with both pat_token and app",
			cfg: Config{
				Orgs: []OrgConfig{func() OrgConfig {
					o := validOrgConfig()
					o.Auth = AuthConfig{
						PATToken: strPtr("ghp_test"),
						GitHubApp: &GitHubAppConfig{
							ClientID:       "Iv23li",
							InstallationID: 123,
							PrivateKeyPath: "/key.pem",
						},
					}
					return o
				}()},
				IdleTimeout: 15 * time.Minute,
			},
			wantErr: "orgs[0].auth: pat_token and github_app are mutually exclusive",
		},
		{
			name: "app auth missing client_id",
			cfg: Config{
				Orgs: []OrgConfig{func() OrgConfig {
					o := validOrgConfig()
					o.Auth = AuthConfig{
						GitHubApp: &GitHubAppConfig{
							InstallationID: 123,
							PrivateKeyPath: "/key.pem",
						},
					}
					return o
				}()},
				IdleTimeout: 15 * time.Minute,
			},
			wantErr: "orgs[0].auth.github_app.client_id is required",
		},
		{
			name: "app auth missing installation_id",
			cfg: Config{
				Orgs: []OrgConfig{func() OrgConfig {
					o := validOrgConfig()
					o.Auth = AuthConfig{
						GitHubApp: &GitHubAppConfig{
							ClientID:       "Iv23li",
							PrivateKeyPath: "/key.pem",
						},
					}
					return o
				}()},
				IdleTimeout: 15 * time.Minute,
			},
			wantErr: "orgs[0].auth.github_app.installation_id is required",
		},
		{
			name: "app auth missing private_key_path",
			cfg: Config{
				Orgs: []OrgConfig{func() OrgConfig {
					o := validOrgConfig()
					o.Auth = AuthConfig{
						GitHubApp: &GitHubAppConfig{
							ClientID:       "Iv23li",
							InstallationID: 123,
						},
					}
					return o
				}()},
				IdleTimeout: 15 * time.Minute,
			},
			wantErr: "orgs[0].auth.github_app.private_key_path is required",
		},
		{
			name: "empty runner_group defaults to Default",
			cfg: Config{
				Orgs: []OrgConfig{func() OrgConfig {
					o := validOrgConfig()
					o.RunnerGroup = ""
					return o
				}()},
				IdleTimeout: 15 * time.Minute,
			},
		},
		{
			name: "org with no runner sets",
			cfg: Config{
				Orgs: []OrgConfig{func() OrgConfig {
					o := validOrgConfig()
					o.RunnerSets = nil
					return o
				}()},
				IdleTimeout: 15 * time.Minute,
			},
			wantErr: "orgs[0].runner_sets must have at least one entry",
		},
		{
			name: "repo missing repo field",
			cfg: Config{
				Repos: []RepoConfig{func() RepoConfig {
					r := validRepoConfig()
					r.Repo = ""
					return r
				}()},
				IdleTimeout: 15 * time.Minute,
			},
			wantErr: "repos[0].repo is required",
		},
		{
			name: "repo with invalid format",
			cfg: Config{
				Repos: []RepoConfig{func() RepoConfig {
					r := validRepoConfig()
					r.Repo = "just-a-name"
					return r
				}()},
				IdleTimeout: 15 * time.Minute,
			},
			wantErr: "repos[0].repo must be in \"owner/repo\" format",
		},
		{
			name: "runner set missing name",
			cfg: Config{
				Orgs: []OrgConfig{func() OrgConfig {
					o := validOrgConfig()
					o.RunnerSets[0].Name = ""
					return o
				}()},
				IdleTimeout: 15 * time.Minute,
			},
			wantErr: "orgs[0].runner_sets[0].name is required",
		},
		{
			name: "duplicate runner set names across orgs and repos",
			cfg: Config{
				Orgs: []OrgConfig{func() OrgConfig {
					o := validOrgConfig()
					o.RunnerSets[0].Name = "duplicate-name"
					return o
				}()},
				Repos: []RepoConfig{func() RepoConfig {
					r := validRepoConfig()
					r.RunnerSets[0].Name = "duplicate-name"
					return r
				}()},
				IdleTimeout: 15 * time.Minute,
			},
			wantErr: "runner set name \"duplicate-name\" is not unique",
		},
		{
			name: "invalid backend",
			cfg: Config{
				Orgs: []OrgConfig{func() OrgConfig {
					o := validOrgConfig()
					o.RunnerSets[0].Backend = "invalid"
					return o
				}()},
				IdleTimeout: 15 * time.Minute,
			},
			wantErr: "must be 'tart' or 'docker'",
		},
		{
			name: "max_runners <= 0",
			cfg: Config{
				Orgs: []OrgConfig{func() OrgConfig {
					o := validOrgConfig()
					o.RunnerSets[0].MaxRunners = 0
					return o
				}()},
				IdleTimeout: 15 * time.Minute,
			},
			wantErr: "max_runners must be > 0",
		},
		{
			name: "idle_timeout <= 0",
			cfg: Config{
				Orgs:        []OrgConfig{validOrgConfig()},
				IdleTimeout: 0,
			},
			wantErr: "idle_timeout must be greater than 0",
		},
		{
			name: "empty pat_token string",
			cfg: Config{
				Orgs: []OrgConfig{func() OrgConfig {
					o := validOrgConfig()
					o.Auth = AuthConfig{PATToken: strPtr("")}
					return o
				}()},
				IdleTimeout: 15 * time.Minute,
			},
			wantErr: "orgs[0].auth.pat_token must not be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.cfg.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("Validate() unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("Validate() expected error containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("Validate() error = %q, want it to contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestValidate_DefaultRunnerGroup(t *testing.T) {
	t.Parallel()

	cfg := Config{
		Orgs: []OrgConfig{func() OrgConfig {
			o := validOrgConfig()
			o.RunnerGroup = ""
			return o
		}()},
		IdleTimeout: 15 * time.Minute,
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() unexpected error: %v", err)
	}

	if cfg.Orgs[0].RunnerGroup != "Default" {
		t.Errorf("RunnerGroup = %q, want %q", cfg.Orgs[0].RunnerGroup, "Default")
	}
}

func TestOrgConfig_ConfigURL(t *testing.T) {
	t.Parallel()

	org := OrgConfig{Org: "boring-design"}
	if got := org.ConfigURL(); got != "https://github.com/boring-design" {
		t.Errorf("ConfigURL() = %q, want %q", got, "https://github.com/boring-design")
	}
}

func TestRepoConfig_ConfigURL(t *testing.T) {
	t.Parallel()

	repo := RepoConfig{Repo: "boring-design/special-repo"}
	if got := repo.ConfigURL(); got != "https://github.com/boring-design/special-repo" {
		t.Errorf("ConfigURL() = %q, want %q", got, "https://github.com/boring-design/special-repo")
	}
}

func TestAuthConfig_Mode(t *testing.T) {
	t.Parallel()

	t.Run("app", func(t *testing.T) {
		t.Parallel()
		auth := validAppAuth()
		if got := auth.Mode(); got != AuthModeGitHubApp {
			t.Errorf("Mode() = %q, want %q", got, AuthModeGitHubApp)
		}
	})

	t.Run("pat", func(t *testing.T) {
		t.Parallel()
		auth := AuthConfig{PATToken: strPtr("ghp_test")}
		if got := auth.Mode(); got != AuthModePAT {
			t.Errorf("Mode() = %q, want %q", got, AuthModePAT)
		}
	})
}
