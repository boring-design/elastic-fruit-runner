package backend

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/boring-design/elastic-fruit-runner/internal/tart"
)

// isRemoteImage returns true if the image looks like a registry reference
// (e.g. "ghcr.io/cirruslabs/macos-tahoe-xcode:26.3") rather than a local
// VM name (e.g. "gha-runner-sequoia-xcode-16").
func isRemoteImage(image string) bool {
	return strings.Contains(image, "/")
}

var tartTracer = otel.Tracer("github.com/boring-design/elastic-fruit-runner/internal/backend/tart")

const (
	runnerDownloadURL = "https://github.com/actions/runner/releases/download/v%s/actions-runner-osx-arm64-%s.tar.gz"

	// envPreserveFailedVMs, when set to a truthy value, makes Cleanup skip
	// `tart stop`/`tart delete` for VMs whose Run() returned an error.
	// Useful for diagnosing macOS launchd / Tart bridge networking issues
	// where the VM appears unreachable: the operator can SSH in manually
	// and inspect the live VM state instead of finding it already deleted.
	// Successful VMs (i.e. those that completed their job) are still cleaned
	// up normally so this is safe to leave on for short debug windows.
	envPreserveFailedVMs = "EFR_TART_PRESERVE_FAILED_VMS"
)

// TartBackend runs each job inside an ephemeral Tart VM.
type TartBackend struct {
	tart              *tart.Manager
	vmImage           string
	logger            *slog.Logger
	preserveFailedVMs bool

	failedMu sync.Mutex
	failed   map[string]struct{}
}

func NewTartBackend(vmImage string) *TartBackend {
	preserve := isTruthy(os.Getenv(envPreserveFailedVMs))
	return &TartBackend{
		tart:              tart.NewManager(),
		vmImage:           vmImage,
		logger:            slog.Default().With("image", vmImage),
		preserveFailedVMs: preserve,
		failed:            make(map[string]struct{}),
	}
}

// isTruthy returns true for common affirmative environment-variable values.
// We deliberately accept a small set of well-known forms so a typo'd value
// like "yes please" does not silently enable a debug-only knob.
func isTruthy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

// markVMFailed records a VM name as belonging to a failed Run() invocation.
// Used by Run on error paths and by Cleanup to decide whether to preserve.
func (b *TartBackend) markVMFailed(name string) {
	b.failedMu.Lock()
	defer b.failedMu.Unlock()
	b.failed[name] = struct{}{}
}

// consumeVMFailed returns whether the named VM was previously marked failed
// and clears the marker. Cleanup is the sole consumer; once a VM is cleaned
// up (or preserved), there is no further use for the marker.
func (b *TartBackend) consumeVMFailed(name string) bool {
	b.failedMu.Lock()
	defer b.failedMu.Unlock()
	_, ok := b.failed[name]
	if ok {
		delete(b.failed, name)
	}
	return ok
}

// Run sets up a Tart VM and starts the GitHub Actions runner inside it.
// It pulls the image if needed, clones, starts the VM, waits for IP,
// downloads the runner binary, and launches it with the JIT config.
//
// On any error, Run records the VM name as failed; if EFR_TART_PRESERVE_FAILED_VMS
// is set, a subsequent Cleanup() will skip stop/delete so an operator can SSH
// into the VM to inspect launchd / Tart bridge networking state.
func (b *TartBackend) Run(ctx context.Context, name, jitConfig string) error {
	ctx, span := tartTracer.Start(ctx, "backend.tart.run",
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
			b.markVMFailed(name)
			return pullErr
		}
	}

	if err := b.tart.Clone(ctx, b.vmImage, name); err != nil {
		err = fmt.Errorf("clone VM: %w", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		b.markVMFailed(name)
		return err
	}
	if err := b.tart.Start(ctx, name); err != nil {
		err = fmt.Errorf("start VM: %w", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		b.markVMFailed(name)
		return err
	}
	if _, err := b.tart.IPAddress(ctx, name); err != nil {
		err = fmt.Errorf("VM unreachable: %w", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		b.markVMFailed(name)
		return err
	}

	version, err := ResolveRunnerVersion(ctx)
	if err != nil {
		err = fmt.Errorf("resolve runner version: %w", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		b.markVMFailed(name)
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
		b.markVMFailed(name)
		return err
	}
	return nil
}

func (b *TartBackend) Cleanup(ctx context.Context, name string) {
	ctx, span := tartTracer.Start(ctx, "backend.tart.cleanup",
		trace.WithAttributes(attribute.String("vm.name", name)),
	)
	defer span.End()

	wasFailed := b.consumeVMFailed(name)
	if wasFailed && b.preserveFailedVMs {
		b.logger.Warn("preserving failed Tart VM for diagnostic inspection",
			"vm", name,
			"env", envPreserveFailedVMs,
			"hint", "run `tart stop "+name+" && tart delete "+name+"` to clean up manually",
		)
		span.SetAttributes(attribute.Bool("vm.preserved", true))
		return
	}

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
