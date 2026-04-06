package config

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoad_Defaults(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test: requires file system")
	}
	t.Setenv("HOME", t.TempDir())

	cfg, err := LoadWithArgs(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.IdleTimeout != 15*time.Minute {
		t.Errorf("IdleTimeout = %v, want %v", cfg.IdleTimeout, 15*time.Minute)
	}
}

func TestLoad_MultiOrgRepoConfigFile(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test: requires file system")
	}
	t.Parallel()

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
	if err := os.WriteFile(cfgFile, []byte(content), 0o644); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	cfg, err := LoadWithArgs([]string{"--config", cfgFile})
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
	if testing.Short() {
		t.Skip("integration test: requires file system")
	}
	t.Parallel()

	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config.yaml")
	content := `idle_timeout: 45m`
	if err := os.WriteFile(cfgFile, []byte(content), 0o644); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	cfg, err := LoadWithArgs([]string{"--config", cfgFile})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.IdleTimeout != 45*time.Minute {
		t.Errorf("IdleTimeout = %v, want %v", cfg.IdleTimeout, 45*time.Minute)
	}
}

func TestLoad_LogLevelFromConfigFile(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test: requires file system")
	}
	t.Parallel()

	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config.yaml")
	content := `log_level: debug`
	if err := os.WriteFile(cfgFile, []byte(content), 0o644); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	cfg, err := LoadWithArgs([]string{"--config", cfgFile})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.LogLevel != "debug" {
		t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, "debug")
	}
	level, err := cfg.ParsedLogLevel()
	if err != nil {
		t.Fatalf("ParsedLogLevel() unexpected error: %v", err)
	}
	if level != slog.LevelDebug {
		t.Errorf("ParsedLogLevel() = %v, want %v", level, slog.LevelDebug)
	}
}

func TestLoad_LogLevelEnvOverride(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test: requires file system")
	}
	t.Setenv("HOME", t.TempDir())
	t.Setenv("LOG_LEVEL", "warn")

	cfg, err := LoadWithArgs(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.LogLevel != "warn" {
		t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, "warn")
	}
}

func TestLoad_LogLevelDefault(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test: requires file system")
	}
	t.Setenv("HOME", t.TempDir())

	cfg, err := LoadWithArgs(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.LogLevel != "info" {
		t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, "info")
	}
}
