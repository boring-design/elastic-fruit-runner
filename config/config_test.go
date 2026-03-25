package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoad_Defaults(t *testing.T) {
	cfg, err := loadWithArgs(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.RunnerGroup != "Default" {
		t.Errorf("RunnerGroup = %q, want %q", cfg.RunnerGroup, "Default")
	}
	if cfg.IdleTimeout != 15*time.Minute {
		t.Errorf("IdleTimeout = %v, want %v", cfg.IdleTimeout, 15*time.Minute)
	}
}

func TestLoad_ConfigFile(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config.yaml")
	content := `
github:
  url: https://github.com/test-org
  token: ghp_testtoken123
runner_group: MyGroup
idle_timeout: 30m
runner_sets:
  - name: test-runner
    backend: docker
    image: test-image:latest
    labels: [self-hosted, Linux]
    max_runners: 3
    platform: linux/amd64
`
	if err := os.WriteFile(cfgFile, []byte(content), 0o644); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	cfg, err := loadWithArgs([]string{"--config", cfgFile})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.GitHub.URL != "https://github.com/test-org" {
		t.Errorf("GitHub.URL = %q, want %q", cfg.GitHub.URL, "https://github.com/test-org")
	}
	if cfg.GitHub.Token != "ghp_testtoken123" {
		t.Errorf("GitHub.Token = %q, want %q", cfg.GitHub.Token, "ghp_testtoken123")
	}
	if cfg.RunnerGroup != "MyGroup" {
		t.Errorf("RunnerGroup = %q, want %q", cfg.RunnerGroup, "MyGroup")
	}
	if cfg.IdleTimeout != 30*time.Minute {
		t.Errorf("IdleTimeout = %v, want %v", cfg.IdleTimeout, 30*time.Minute)
	}
	if len(cfg.RunnerSets) != 1 {
		t.Fatalf("len(RunnerSets) = %d, want 1", len(cfg.RunnerSets))
	}
	rs := cfg.RunnerSets[0]
	if rs.Name != "test-runner" {
		t.Errorf("RunnerSets[0].Name = %q, want %q", rs.Name, "test-runner")
	}
	if rs.Backend != "docker" {
		t.Errorf("RunnerSets[0].Backend = %q, want %q", rs.Backend, "docker")
	}
	if rs.Image != "test-image:latest" {
		t.Errorf("RunnerSets[0].Image = %q, want %q", rs.Image, "test-image:latest")
	}
	if rs.MaxRunners != 3 {
		t.Errorf("RunnerSets[0].MaxRunners = %d, want %d", rs.MaxRunners, 3)
	}
	if rs.Platform != "linux/amd64" {
		t.Errorf("RunnerSets[0].Platform = %q, want %q", rs.Platform, "linux/amd64")
	}
	if len(rs.Labels) != 2 || rs.Labels[0] != "self-hosted" || rs.Labels[1] != "Linux" {
		t.Errorf("RunnerSets[0].Labels = %v, want [self-hosted Linux]", rs.Labels)
	}
}

func TestLoad_EnvOverridesFile(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config.yaml")
	content := `
github:
  url: https://github.com/file-org
  token: ghp_from_file
runner_group: FileGroup
`
	if err := os.WriteFile(cfgFile, []byte(content), 0o644); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	t.Setenv("GITHUB_TOKEN", "ghp_from_env")
	t.Setenv("GITHUB_RUNNER_GROUP", "EnvGroup")

	cfg, err := loadWithArgs([]string{"--config", cfgFile})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.GitHub.Token != "ghp_from_env" {
		t.Errorf("GitHub.Token = %q, want %q (env should override file)", cfg.GitHub.Token, "ghp_from_env")
	}
	if cfg.RunnerGroup != "EnvGroup" {
		t.Errorf("RunnerGroup = %q, want %q (env should override file)", cfg.RunnerGroup, "EnvGroup")
	}
	// URL should remain from file since GITHUB_CONFIG_URL is not set
	if cfg.GitHub.URL != "https://github.com/file-org" {
		t.Errorf("GitHub.URL = %q, want %q (should keep file value)", cfg.GitHub.URL, "https://github.com/file-org")
	}
}

func TestLoad_FlagsOverrideEnv(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config.yaml")
	content := `
github:
  url: https://github.com/file-org
  token: ghp_from_file
`
	if err := os.WriteFile(cfgFile, []byte(content), 0o644); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	t.Setenv("GITHUB_CONFIG_URL", "https://github.com/env-org")
	t.Setenv("GITHUB_TOKEN", "ghp_from_env")

	cfg, err := loadWithArgs([]string{
		"--config", cfgFile,
		"--url", "https://github.com/flag-org",
		"--token", "ghp_from_flag",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.GitHub.URL != "https://github.com/flag-org" {
		t.Errorf("GitHub.URL = %q, want %q (flag should override env)", cfg.GitHub.URL, "https://github.com/flag-org")
	}
	if cfg.GitHub.Token != "ghp_from_flag" {
		t.Errorf("GitHub.Token = %q, want %q (flag should override env)", cfg.GitHub.Token, "ghp_from_flag")
	}
}

func TestLoad_DurationParsing(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config.yaml")
	content := `idle_timeout: 30m`
	if err := os.WriteFile(cfgFile, []byte(content), 0o644); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	cfg, err := loadWithArgs([]string{"--config", cfgFile})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.IdleTimeout != 30*time.Minute {
		t.Errorf("IdleTimeout = %v, want %v", cfg.IdleTimeout, 30*time.Minute)
	}
}

func TestLoad_RunnerSets(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config.yaml")
	content := `
runner_sets:
  - name: macos-runner
    backend: tart
    image: macos-image:latest
    labels: [self-hosted, macOS, ARM64]
    max_runners: 2
  - name: linux-runner
    backend: docker
    image: linux-image:latest
    labels: [self-hosted, Linux]
    max_runners: 4
    platform: linux/arm64
`
	if err := os.WriteFile(cfgFile, []byte(content), 0o644); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	cfg, err := loadWithArgs([]string{"--config", cfgFile})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cfg.RunnerSets) != 2 {
		t.Fatalf("len(RunnerSets) = %d, want 2", len(cfg.RunnerSets))
	}
	if cfg.RunnerSets[0].Name != "macos-runner" {
		t.Errorf("RunnerSets[0].Name = %q, want %q", cfg.RunnerSets[0].Name, "macos-runner")
	}
	if cfg.RunnerSets[1].Platform != "linux/arm64" {
		t.Errorf("RunnerSets[1].Platform = %q, want %q", cfg.RunnerSets[1].Platform, "linux/arm64")
	}
}

func TestLoad_GitHubAppConfig(t *testing.T) {
	t.Setenv("GITHUB_APP_CLIENT_ID", "Iv1.test123")
	t.Setenv("GITHUB_APP_INSTALLATION_ID", "12345678")
	t.Setenv("GITHUB_APP_PRIVATE_KEY_PATH", "/path/to/key.pem")

	cfg, err := loadWithArgs(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.GitHub.App.ClientID != "Iv1.test123" {
		t.Errorf("GitHub.App.ClientID = %q, want %q", cfg.GitHub.App.ClientID, "Iv1.test123")
	}
	if cfg.GitHub.App.InstallationID != 12345678 {
		t.Errorf("GitHub.App.InstallationID = %d, want %d", cfg.GitHub.App.InstallationID, 12345678)
	}
	if cfg.GitHub.App.PrivateKeyPath != "/path/to/key.pem" {
		t.Errorf("GitHub.App.PrivateKeyPath = %q, want %q", cfg.GitHub.App.PrivateKeyPath, "/path/to/key.pem")
	}
}

func TestLoad_DotenvFile(t *testing.T) {
	// godotenv.Load() reads .env from the current working directory,
	// so chdir to a temp dir with a .env file
	dir := t.TempDir()
	envFile := filepath.Join(dir, ".env")
	content := `GITHUB_CONFIG_URL=https://github.com/dotenv-org
GITHUB_TOKEN=ghp_from_dotenv
`
	if err := os.WriteFile(envFile, []byte(content), 0o644); err != nil {
		t.Fatalf("write dotenv file: %v", err)
	}

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() { os.Chdir(origDir) })
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	cfg, err := loadWithArgs(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.GitHub.URL != "https://github.com/dotenv-org" {
		t.Errorf("GitHub.URL = %q, want %q", cfg.GitHub.URL, "https://github.com/dotenv-org")
	}
	if cfg.GitHub.Token != "ghp_from_dotenv" {
		t.Errorf("GitHub.Token = %q, want %q", cfg.GitHub.Token, "ghp_from_dotenv")
	}
}
