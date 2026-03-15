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

// templateDir is the shared directory where the runner binary is downloaded once.
func (b *HostBackend) templateDir() string {
	return filepath.Join(b.basePath, "runner")
}

// instanceDir returns the per-runner copy of the runner binary directory.
// Each runner gets its own copy so multiple runners can run concurrently
// without conflicting on lock files and state.
func (b *HostBackend) instanceDir(name string) string {
	return filepath.Join(b.basePath, "instances", name)
}

func (b *HostBackend) Prepare(ctx context.Context, name string) error {
	tmpl := b.templateDir()
	if err := b.ensureRunner(ctx, tmpl); err != nil {
		return fmt.Errorf("ensure runner binary: %w", err)
	}

	inst := b.instanceDir(name)
	b.logger.Info("copying runner template for instance", "src", tmpl, "dst", inst)

	cmd := exec.CommandContext(ctx, "cp", "-a", tmpl, inst)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("copy runner template: %w", err)
	}
	return nil
}

func (b *HostBackend) RunRunner(ctx context.Context, name, jitConfig string) error {
	inst := b.instanceDir(name)

	b.logger.Info("starting runner on host", "runner", name, "dir", inst)

	runPath := filepath.Join(inst, "run.sh")
	runCmd := exec.CommandContext(ctx, runPath,
		"--jitconfig", jitConfig,
	)
	runCmd.Dir = inst
	runCmd.Stdout = os.Stdout
	runCmd.Stderr = os.Stderr
	if err := runCmd.Run(); err != nil {
		return fmt.Errorf("run runner: %w", err)
	}

	return nil
}

func (b *HostBackend) Cleanup(_ context.Context, name string) {
	dir := b.instanceDir(name)
	b.logger.Info("cleaning up runner instance", "dir", dir)
	if err := os.RemoveAll(dir); err != nil {
		b.logger.Warn("cleanup runner instance", "dir", dir, "err", err)
	}
}

// ensureRunner downloads the actions/runner binary if not already present.
func (b *HostBackend) ensureRunner(ctx context.Context, dir string) error {
	runSh := filepath.Join(dir, "run.sh")
	if _, err := os.Stat(runSh); err == nil {
		return nil
	}

	version, err := ResolveRunnerVersion(ctx)
	if err != nil {
		return fmt.Errorf("resolve runner version: %w", err)
	}

	b.logger.Info("downloading actions/runner", "version", version, "dir", dir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	archSuffix := runnerArchSuffix()
	url := fmt.Sprintf(
		"https://github.com/actions/runner/releases/download/v%s/actions-runner-%s-%s.tar.gz",
		version, archSuffix, version,
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
