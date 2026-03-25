package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func strPtr(s string) *string { return &s }

func TestLoad_Defaults(t *testing.T) {
	cfg, err := loadWithArgs(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.IdleTimeout != 15*time.Minute {
		t.Errorf("IdleTimeout = %v, want %v", cfg.IdleTimeout, 15*time.Minute)
	}
}

func TestLoad_MultiOrgRepoConfigFile(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config.yaml")
	content := `
orgs:
  - org: boring-design
    auth:
      github_app:
        client_id: Iv23li_test
        installation_id: 116416405
        private_key_path: /path/to/key.pem
    runner_group: MyGroup
    runner_sets:
      - name: efr-linux-arm64
        backend: docker
        image: test-image:latest
        labels: [self-hosted, Linux, ARM64]
        max_runners: 4
        platform: linux/arm64

repos:
  - repo: boring-design/special-repo
    auth:
      pat_token: ghp_testtoken123
    runner_sets:
      - name: repo-runner
        backend: docker
        image: repo-image:latest
        labels: [self-hosted, Linux]
        max_runners: 2

idle_timeout: 30m
`
	if err := os.WriteFile(cfgFile, []byte(content), 0644); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	cfg, err := loadWithArgs([]string{"--config", cfgFile})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.IdleTimeout != 30*time.Minute {
		t.Errorf("IdleTimeout = %v, want %v", cfg.IdleTimeout, 30*time.Minute)
	}

	if len(cfg.Orgs) != 1 {
		t.Fatalf("len(Orgs) = %d, want 1", len(cfg.Orgs))
	}
	org := cfg.Orgs[0]
	if org.Org != "boring-design" {
		t.Errorf("Orgs[0].Org = %q, want %q", org.Org, "boring-design")
	}
	if org.RunnerGroup != "MyGroup" {
		t.Errorf("Orgs[0].RunnerGroup = %q, want %q", org.RunnerGroup, "MyGroup")
	}
	if org.Auth.GitHubApp == nil {
		t.Fatal("Orgs[0].Auth.GitHubApp is nil")
	}
	if org.Auth.GitHubApp.ClientID != "Iv23li_test" {
		t.Errorf("Orgs[0].Auth.GitHubApp.ClientID = %q, want %q", org.Auth.GitHubApp.ClientID, "Iv23li_test")
	}
	if org.Auth.GitHubApp.InstallationID != 116416405 {
		t.Errorf("Orgs[0].Auth.GitHubApp.InstallationID = %d, want %d", org.Auth.GitHubApp.InstallationID, 116416405)
	}
	if org.Auth.GitHubApp.PrivateKeyPath != "/path/to/key.pem" {
		t.Errorf("Orgs[0].Auth.GitHubApp.PrivateKeyPath = %q, want %q", org.Auth.GitHubApp.PrivateKeyPath, "/path/to/key.pem")
	}
	if len(org.RunnerSets) != 1 {
		t.Fatalf("len(Orgs[0].RunnerSets) = %d, want 1", len(org.RunnerSets))
	}
	rs := org.RunnerSets[0]
	if rs.Name != "efr-linux-arm64" {
		t.Errorf("Orgs[0].RunnerSets[0].Name = %q, want %q", rs.Name, "efr-linux-arm64")
	}
	if rs.Backend != "docker" {
		t.Errorf("Orgs[0].RunnerSets[0].Backend = %q, want %q", rs.Backend, "docker")
	}
	if rs.MaxRunners != 4 {
		t.Errorf("Orgs[0].RunnerSets[0].MaxRunners = %d, want %d", rs.MaxRunners, 4)
	}
	if rs.Platform != "linux/arm64" {
		t.Errorf("Orgs[0].RunnerSets[0].Platform = %q, want %q", rs.Platform, "linux/arm64")
	}

	if len(cfg.Repos) != 1 {
		t.Fatalf("len(Repos) = %d, want 1", len(cfg.Repos))
	}
	repo := cfg.Repos[0]
	if repo.Repo != "boring-design/special-repo" {
		t.Errorf("Repos[0].Repo = %q, want %q", repo.Repo, "boring-design/special-repo")
	}
	if repo.Auth.PATToken == nil {
		t.Fatal("Repos[0].Auth.PATToken is nil")
	}
	if *repo.Auth.PATToken != "ghp_testtoken123" {
		t.Errorf("Repos[0].Auth.PATToken = %q, want %q", *repo.Auth.PATToken, "ghp_testtoken123")
	}
	if len(repo.RunnerSets) != 1 {
		t.Fatalf("len(Repos[0].RunnerSets) = %d, want 1", len(repo.RunnerSets))
	}
	rrs := repo.RunnerSets[0]
	if rrs.Name != "repo-runner" {
		t.Errorf("Repos[0].RunnerSets[0].Name = %q, want %q", rrs.Name, "repo-runner")
	}
}

func TestLoad_DurationParsing(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config.yaml")
	content := `idle_timeout: 45m`
	if err := os.WriteFile(cfgFile, []byte(content), 0644); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	cfg, err := loadWithArgs([]string{"--config", cfgFile})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.IdleTimeout != 45*time.Minute {
		t.Errorf("IdleTimeout = %v, want %v", cfg.IdleTimeout, 45*time.Minute)
	}
}

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

func TestValidate(t *testing.T) {
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
			wantErr: "must be 'tart', 'docker', or 'host'",
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
	org := OrgConfig{Org: "boring-design"}
	if got := org.ConfigURL(); got != "https://github.com/boring-design" {
		t.Errorf("ConfigURL() = %q, want %q", got, "https://github.com/boring-design")
	}
}

func TestRepoConfig_ConfigURL(t *testing.T) {
	repo := RepoConfig{Repo: "boring-design/special-repo"}
	if got := repo.ConfigURL(); got != "https://github.com/boring-design/special-repo" {
		t.Errorf("ConfigURL() = %q, want %q", got, "https://github.com/boring-design/special-repo")
	}
}

func TestAuthConfig_Mode(t *testing.T) {
	t.Run("app", func(t *testing.T) {
		auth := validAppAuth()
		if got := auth.Mode(); got != AuthModeGitHubApp {
			t.Errorf("Mode() = %q, want %q", got, AuthModeGitHubApp)
		}
	})

	t.Run("pat", func(t *testing.T) {
		auth := AuthConfig{PATToken: strPtr("ghp_test")}
		if got := auth.Mode(); got != AuthModePAT {
			t.Errorf("Mode() = %q, want %q", got, AuthModePAT)
		}
	})
}

