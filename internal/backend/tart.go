package backend

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/boring-design/elastic-fruit-runner/internal/tart"
)

// isRemoteImage returns true if the image looks like a registry reference
// (e.g. "ghcr.io/cirruslabs/macos-sequoia-base:latest") rather than a local
// VM name (e.g. "gha-runner-sequoia-xcode-16").
func isRemoteImage(image string) bool {
	return strings.Contains(image, "/")
}

var tartTracer = otel.Tracer("github.com/boring-design/elastic-fruit-runner/internal/backend/tart")

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
	ctx, span := tartTracer.Start(ctx, "backend.tart.prepare",
		trace.WithAttributes(attribute.String("vm.name", name)),
	)
	defer span.End()

	exists, err := b.tart.ImageExists(ctx, b.vmImage)
	if err != nil {
		b.logger.Warn("failed to check image existence, proceeding with clone", "err", err)
	}
	if !exists && isRemoteImage(b.vmImage) {
		b.logger.Info("VM image not found locally, pulling", "image", b.vmImage)
		if pullErr := b.tart.Pull(ctx, b.vmImage); pullErr != nil {
			pullErr = fmt.Errorf("pull VM image: %w", pullErr)
			span.RecordError(pullErr)
			span.SetStatus(codes.Error, pullErr.Error())
			return pullErr
		}
	}

	if err := b.tart.Clone(ctx, b.vmImage, name); err != nil {
		err = fmt.Errorf("clone VM: %w", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	if err := b.tart.Start(ctx, name); err != nil {
		err = fmt.Errorf("start VM: %w", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	if _, err := b.tart.IPAddress(ctx, name); err != nil {
		err = fmt.Errorf("VM unreachable: %w", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	return nil
}

// StartRunner downloads (if needed) and starts the GitHub Actions runner inside
// the VM with the given JIT config. The runner process is started in the
// background via nohup, so this call returns as soon as the process is launched.
func (b *TartBackend) StartRunner(ctx context.Context, name, jitConfig string) error {
	ctx, span := tartTracer.Start(ctx, "backend.tart.start_runner",
		trace.WithAttributes(attribute.String("vm.name", name)),
	)
	defer span.End()

	version, err := ResolveRunnerVersion(ctx)
	if err != nil {
		err = fmt.Errorf("resolve runner version: %w", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	span.SetAttributes(attribute.String("runner.version", version))

	url := fmt.Sprintf(runnerDownloadURL, version, version)

	// Download runner binary if not already present, then launch it in the
	// background. nohup ensures the runner process outlives this tart exec call.
	script := fmt.Sprintf(`
set -euo pipefail
mkdir -p ~/actions-runner && cd ~/actions-runner

if [ ! -f ./run.sh ]; then
  echo "Downloading actions/runner %s..."
  curl -fsSL -o runner.tar.gz "%s"
  tar xzf runner.tar.gz
  rm runner.tar.gz
fi

nohup ./run.sh --jitconfig "%s" > /tmp/runner.log 2>&1 &
`, version, url, jitConfig)

	if err := b.tart.Exec(ctx, name, "bash", "-c", script); err != nil {
		err = fmt.Errorf("start runner in VM %s: %w", name, err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	return nil
}

func (b *TartBackend) Cleanup(ctx context.Context, name string) {
	ctx, span := tartTracer.Start(ctx, "backend.tart.cleanup",
		trace.WithAttributes(attribute.String("vm.name", name)),
	)
	defer span.End()

	if err := b.tart.Stop(ctx, name); err != nil {
		b.logger.Warn("stop VM", "vm", name, "err", err)
		span.RecordError(err)
	}
	if err := b.tart.Delete(ctx, name); err != nil {
		b.logger.Warn("delete VM", "vm", name, "err", err)
		span.RecordError(err)
	}
}

func (b *TartBackend) CleanupAll(ctx context.Context, prefix string) {
	_, span := tartTracer.Start(ctx, "backend.tart.cleanup_all",
		trace.WithAttributes(attribute.String("prefix", prefix)),
	)
	defer span.End()

	vms, err := b.tart.List(ctx)
	if err != nil {
		b.logger.Warn("list VMs for cleanup", "prefix", prefix, "err", err)
		return
	}

	for _, name := range vms {
		if strings.HasPrefix(name, prefix+"-") {
			b.logger.Info("removing orphaned VM", "vm", name)
			b.Cleanup(ctx, name)
		}
	}
}
