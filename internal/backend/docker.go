package backend

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"

	"github.com/boring-design/elastic-fruit-runner/internal/binpath"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

var dockerTracer = otel.Tracer("github.com/boring-design/elastic-fruit-runner/internal/backend/docker")

const defaultDockerRunnerImage = "ghcr.io/actions-runner-controller/actions-runner-controller/actions-runner-dind:latest"

// DockerBackend runs each job inside an ephemeral Docker container.
type DockerBackend struct {
	image    string
	platform string
	logger   *slog.Logger
}

func NewDockerBackend(image, platform string, logger *slog.Logger) *DockerBackend {
	if image == "" {
		image = defaultDockerRunnerImage
	}
	return &DockerBackend{
		image:    image,
		platform: platform,
		logger:   logger,
	}
}

// initScript is the container entrypoint that mirrors ARC's entrypoint-dind.sh:
// 1. Start dockerd under root (with proper redirect permissions)
// 2. Wait up to 30s for dockerd to be ready (like ARC's wait.sh)
// 3. Sleep forever — runner is started later via docker exec
const initScript = `
sudo /usr/bin/dockerd &
for i in $(seq 1 30); do
  if pgrep -x dockerd > /dev/null 2>&1; then break; fi
  sleep 1
done
sleep infinity
`

// startRunnerScript mirrors ARC's startup.sh:
// 1. Copy runner assets from /runnertmp to /home/runner (like startup.sh copies to RUNNER_HOME)
// 2. Wait for Docker socket to be ready (like startup.sh's docker wait loop)
// 3. Start the runner with JIT config
const startRunnerScript = `
set -e
cp -a /runnertmp/. /home/runner/
cd /home/runner
for i in $(seq 1 30); do
  if docker info &>/dev/null; then break; fi
  sleep 1
done
exec ./run.sh --jitconfig "$1"
`

func (b *DockerBackend) Prepare(ctx context.Context, name string) error {
	ctx, span := dockerTracer.Start(ctx, "backend.docker.prepare",
		trace.WithAttributes(attribute.String("container.name", name)),
	)
	defer span.End()

	args := []string{"run", "-d", "--privileged", "--name", name}
	if b.platform != "" {
		args = append(args, "--platform", b.platform)
	}
	args = append(args, "--entrypoint", "bash", b.image, "-c", initScript)

	cmd := exec.CommandContext(ctx, binpath.Lookup("docker"), args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		err = fmt.Errorf("docker run: %s: %w", string(out), err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	return nil
}

func (b *DockerBackend) StartRunner(ctx context.Context, name, jitConfig string) error {
	ctx, span := dockerTracer.Start(ctx, "backend.docker.start_runner",
		trace.WithAttributes(attribute.String("container.name", name)),
	)
	defer span.End()

	cmd := exec.CommandContext(ctx, binpath.Lookup("docker"), "exec", "-d", name,
		"bash", "-c", startRunnerScript, "bash", jitConfig,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		err = fmt.Errorf("docker exec start runner in %s: %s: %w", name, string(out), err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	return nil
}

func (b *DockerBackend) Cleanup(ctx context.Context, name string) {
	_, span := dockerTracer.Start(ctx, "backend.docker.cleanup",
		trace.WithAttributes(attribute.String("container.name", name)),
	)
	defer span.End()

	cmd := exec.CommandContext(ctx, binpath.Lookup("docker"), "rm", "-f", name)
	if out, err := cmd.CombinedOutput(); err != nil {
		b.logger.Warn("docker rm", "container", name, "err", err, "output", string(out))
		span.RecordError(err)
	}
}

func (b *DockerBackend) CleanupAll(ctx context.Context, prefix string) {
	_, span := dockerTracer.Start(ctx, "backend.docker.cleanup_all",
		trace.WithAttributes(attribute.String("prefix", prefix)),
	)
	defer span.End()

	cmd := exec.CommandContext(ctx, binpath.Lookup("docker"), "ps", "-a",
		"--filter", fmt.Sprintf("name=^%s-", prefix),
		"--format", "{{.Names}}",
	)
	out, err := cmd.Output()
	if err != nil {
		b.logger.Warn("docker ps for cleanup", "prefix", prefix, "err", err)
		return
	}

	names := strings.TrimSpace(string(out))
	if names == "" {
		return
	}

	for _, name := range strings.Split(names, "\n") {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		b.logger.Info("removing orphaned container", "container", name)
		b.Cleanup(ctx, name)
	}
}
