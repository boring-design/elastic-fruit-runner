package backend

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// HostBackend runs the GitHub Actions runner directly on the host machine.
// Each job gets its own working directory under a base path, which is
// removed after the job completes.
type HostBackend struct {
	basePath string
	logger   *slog.Logger
}

func NewHostBackend(logger *slog.Logger) *HostBackend {
	home, _ := os.UserHomeDir()
	return &HostBackend{
		basePath: filepath.Join(home, ".elastic-fruit-runner"),
		logger:   logger,
	}
}

func (b *HostBackend) runnerDir() string {
	return filepath.Join(b.basePath, "runner")
}

func (b *HostBackend) workDir(name string) string {
	return filepath.Join(b.basePath, "work", name)
}

func (b *HostBackend) Prepare(_ context.Context, name string) error {
	dir := b.workDir(name)
	b.logger.Info("preparing host work directory", "dir", dir)
	return os.MkdirAll(dir, 0o755)
}

func (b *HostBackend) RunRunner(ctx context.Context, name, jitConfig string) error {
	runnerDir := b.runnerDir()
	workDir := b.workDir(name)

	if err := b.ensureRunner(ctx, runnerDir); err != nil {
		return fmt.Errorf("ensure runner binary: %w", err)
	}

	b.logger.Info("starting runner on host", "runner", name, "workDir", workDir)

	configPath := filepath.Join(runnerDir, "config.sh")
	configCmd := exec.CommandContext(ctx, configPath,
		"--unattended",
		"--jitconfig", jitConfig,
		"--work", workDir,
	)
	configCmd.Dir = runnerDir
	configCmd.Stdout = os.Stdout
	configCmd.Stderr = os.Stderr
	if err := configCmd.Run(); err != nil {
		return fmt.Errorf("configure runner: %w", err)
	}

	runPath := filepath.Join(runnerDir, "run.sh")
	runCmd := exec.CommandContext(ctx, runPath)
	runCmd.Dir = runnerDir
	runCmd.Stdout = os.Stdout
	runCmd.Stderr = os.Stderr
	if err := runCmd.Run(); err != nil {
		return fmt.Errorf("run runner: %w", err)
	}

	return nil
}

func (b *HostBackend) Cleanup(_ context.Context, name string) {
	dir := b.workDir(name)
	b.logger.Info("cleaning up work directory", "dir", dir)
	if err := os.RemoveAll(dir); err != nil {
		b.logger.Warn("cleanup work directory", "dir", dir, "err", err)
	}
}

// ensureRunner downloads the actions/runner binary if not already present.
func (b *HostBackend) ensureRunner(ctx context.Context, dir string) error {
	runSh := filepath.Join(dir, "run.sh")
	if _, err := os.Stat(runSh); err == nil {
		return nil
	}

	b.logger.Info("downloading actions/runner", "version", runnerVersion, "dir", dir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	archSuffix := runnerArchSuffix()
	url := fmt.Sprintf(
		"https://github.com/actions/runner/releases/download/v%s/actions-runner-%s-%s.tar.gz",
		runnerVersion, archSuffix, runnerVersion,
	)

	cmd := exec.CommandContext(ctx, "bash", "-c",
		fmt.Sprintf(`curl -fsSL -o runner.tar.gz "%s" && tar xzf runner.tar.gz && rm runner.tar.gz`, url),
	)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func runnerArchSuffix() string {
	os := runtime.GOOS
	arch := runtime.GOARCH

	switch {
	case os == "darwin" && arch == "arm64":
		return "osx-arm64"
	case os == "darwin" && arch == "amd64":
		return "osx-x64"
	case os == "linux" && arch == "arm64":
		return "linux-arm64"
	case os == "linux" && arch == "amd64":
		return "linux-x64"
	default:
		return fmt.Sprintf("%s-%s", os, arch)
	}
}
