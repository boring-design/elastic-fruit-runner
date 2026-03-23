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

// Run starts a DinD container with the native entrypoint-dind.sh.
// The JIT config is passed via ACTIONS_RUNNER_INPUT_JITCONFIG env var,
// which the GitHub Actions runner picks up automatically — mirroring
// how ARC passes JIT config to runner pods in Kubernetes.
func (b *DockerBackend) Run(ctx context.Context, name, jitConfig string) error {
	ctx, span := dockerTracer.Start(ctx, "backend.docker.run",
		trace.WithAttributes(attribute.String("container.name", name)),
	)
	defer span.End()

	args := []string{
		"run", "-d", "--privileged",
		"--name", name,
		"-e", "ACTIONS_RUNNER_INPUT_JITCONFIG=" + jitConfig,
	}
	if b.platform != "" {
		args = append(args, "--platform", b.platform)
	}
	args = append(args, b.image)

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
