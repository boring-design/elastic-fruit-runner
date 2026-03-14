package backend

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/boring-design/elastic-fruit-runner/internal/tart"
)

const (
	runnerDownloadURL = "https://github.com/actions/runner/releases/download/v%s/actions-runner-osx-arm64-%s.tar.gz"
)

// TartBackend runs each job inside an ephemeral Tart VM.
type TartBackend struct {
	tart    *tart.Manager
	vmImage string
	logger  *slog.Logger
}

func NewTartBackend(vmImage string, logger *slog.Logger) *TartBackend {
	return &TartBackend{
		tart:    tart.NewManager(logger),
		vmImage: vmImage,
		logger:  logger,
	}
}

func (b *TartBackend) Prepare(ctx context.Context, name string) error {
	if err := b.tart.Clone(ctx, b.vmImage, name); err != nil {
		return fmt.Errorf("clone VM: %w", err)
	}
	if err := b.tart.Start(ctx, name); err != nil {
		return fmt.Errorf("start VM: %w", err)
	}
	if _, err := b.tart.IPAddress(ctx, name); err != nil {
		return fmt.Errorf("VM unreachable: %w", err)
	}
	return nil
}

func (b *TartBackend) RunRunner(ctx context.Context, name, jitConfig string) error {
	version, err := ResolveRunnerVersion(ctx)
	if err != nil {
		return fmt.Errorf("resolve runner version: %w", err)
	}

	url := fmt.Sprintf(runnerDownloadURL, version, version)

	script := fmt.Sprintf(`
set -euo pipefail
mkdir -p ~/actions-runner && cd ~/actions-runner

if [ ! -f ./run.sh ]; then
  echo "Downloading actions/runner %s..."
  curl -fsSL -o runner.tar.gz "%s"
  tar xzf runner.tar.gz
  rm runner.tar.gz
fi

./run.sh --jitconfig "%s"
`, version, url, jitConfig)

	out, err := b.tart.ExecOutput(ctx, name, "bash", "-c", script)
	if err != nil {
		b.logger.Error("runner script failed", "vm", name, "output", string(out), "err", err)
		return fmt.Errorf("runner in VM %s: %w", name, err)
	}
	return nil
}

func (b *TartBackend) Cleanup(ctx context.Context, name string) {
	if err := b.tart.Stop(ctx, name); err != nil {
		b.logger.Warn("stop VM", "vm", name, "err", err)
	}
	if err := b.tart.Delete(ctx, name); err != nil {
		b.logger.Warn("delete VM", "vm", name, "err", err)
	}
}
