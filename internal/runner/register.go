package runner

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/boring-design/elastic-fruit-runner/internal/tart"
)

const (
	// runnerVersion is the actions/runner release to install inside each VM.
	// Update this when a new stable release is available.
	runnerVersion = "2.322.0"

	// runnerDownloadURL is the ARM64 macOS runner tarball URL template.
	runnerDownloadURL = "https://github.com/actions/runner/releases/download/v%s/actions-runner-osx-arm64-%s.tar.gz"
)

// Registrar handles downloading, configuring, and starting the actions/runner
// binary inside a Tart VM using a JIT (Just-In-Time) configuration token.
type Registrar struct {
	tart   *tart.Manager
	logger *slog.Logger
}

func NewRegistrar(t *tart.Manager, logger *slog.Logger) *Registrar {
	return &Registrar{tart: t, logger: logger}
}

// StartWithJITConfig downloads the actions/runner binary into the VM (if not
// already present), configures it with the provided JIT config, and runs it.
// This call blocks until the runner exits (i.e. the job finishes).
func (r *Registrar) StartWithJITConfig(ctx context.Context, vmName, jitConfig string) error {
	r.logger.Info("starting runner in VM", "vm", vmName)

	url := fmt.Sprintf(runnerDownloadURL, runnerVersion, runnerVersion)

	// One-liner setup script: idempotent download + configure + run.
	// The --jitconfig flag registers an ephemeral runner that accepts exactly
	// one job and then exits, making cleanup straightforward.
	script := fmt.Sprintf(`
set -euo pipefail
mkdir -p ~/actions-runner && cd ~/actions-runner

# Download runner only if not already present (supports pre-warmed images).
if [ ! -f ./run.sh ]; then
  echo "Downloading actions/runner %s..."
  curl -fsSL -o runner.tar.gz "%s"
  tar xzf runner.tar.gz
  rm runner.tar.gz
fi

# Configure and run with JIT config (ephemeral — runs exactly one job).
./config.sh --unattended --jitconfig "%s"
./run.sh
`, runnerVersion, url, jitConfig)

	out, err := r.tart.ExecOutput(ctx, vmName, "bash", "-c", script)
	if err != nil {
		r.logger.Error("runner script failed", "vm", vmName, "output", string(out), "err", err)
		return fmt.Errorf("runner in VM %s: %w", vmName, err)
	}

	r.logger.Info("runner exited cleanly", "vm", vmName)
	return nil
}
