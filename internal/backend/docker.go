package backend

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/boring-design/elastic-fruit-runner/internal/binpath"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type dockerStats struct {
	CPUPercent string `json:"CPUPerc"`
	Memory     string `json:"MemUsage"`
	Network    string `json:"NetIO"`
	Block      string `json:"BlockIO"`
}

var dockerTracer = otel.Tracer("github.com/boring-design/elastic-fruit-runner/internal/backend/docker")

const defaultDockerRunnerImage = "ghcr.io/quipper/actions-runner:2.332.0"

// DockerBackend runs each job inside an ephemeral Docker container.
type DockerBackend struct {
	image    string
	platform string
	logger   *slog.Logger
}

func NewDockerBackend(image, platform string) *DockerBackend {
	if image == "" {
		image = defaultDockerRunnerImage
	}
	logger := slog.Default().With("image", image)
	if platform != "" {
		logger = logger.With("platform", platform)
	}
	return &DockerBackend{
		image:    image,
		platform: platform,
		logger:   logger,
	}
}

// Run starts a DinD container and launches the GitHub Actions runner.
//
// Uses the quipper/actions-runner image (github.com/quipper/actions-runner)
// whose entrypoint unconditionally starts dockerd, then execs CMD
// (/home/runner/run.sh) which reads ACTIONS_RUNNER_INPUT_JITCONFIG.
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

func (b *DockerBackend) ReadLogs(ctx context.Context, name string) (string, error) {
	cmd := exec.CommandContext(ctx, binpath.Lookup("docker"), "logs", name)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("read Docker logs for %s: %w: %s", name, err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

func (b *DockerBackend) ReadResource(ctx context.Context, name string) (ResourceSample, error) {
	cmd := exec.CommandContext(ctx, binpath.Lookup("docker"), "stats", "--no-stream", "--format", "{{json .}}", name)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return ResourceSample{}, fmt.Errorf("read Docker resource data for %s: %w: %s", name, err, strings.TrimSpace(string(out)))
	}
	var stats dockerStats
	if err := json.Unmarshal(out, &stats); err != nil {
		return ResourceSample{}, fmt.Errorf("parse Docker resource data for %s: %w", name, err)
	}
	memoryUsed, memoryAvailable := parsePair(stats.Memory)
	networkReceive, networkSend := parsePair(stats.Network)
	diskRead, diskWrite := parsePair(stats.Block)
	cpu, _ := strconv.ParseFloat(strings.TrimSuffix(strings.TrimSpace(stats.CPUPercent), "%"), 64)
	return ResourceSample{
		RecordedAt:           time.Now(),
		Source:               "docker",
		Accuracy:             "exact",
		CPUPercent:           cpu,
		MemoryUsedBytes:      memoryUsed,
		MemoryAvailableBytes: memoryAvailable,
		DiskReadBytes:        diskRead,
		DiskWriteBytes:       diskWrite,
		NetworkReceiveBytes:  networkReceive,
		NetworkSendBytes:     networkSend,
	}, nil
}

func parsePair(value string) (first, second int64) {
	parts := strings.Split(value, "/")
	if len(parts) != 2 {
		return 0, 0
	}
	return parseSize(parts[0]), parseSize(parts[1])
}

func parseSize(value string) int64 {
	value = strings.TrimSpace(value)
	units := []struct {
		name   string
		factor float64
	}{
		{"KiB", 1024}, {"MiB", 1024 * 1024}, {"GiB", 1024 * 1024 * 1024},
		{"TiB", 1024 * 1024 * 1024 * 1024}, {"kB", 1000}, {"KB", 1000},
		{"MB", 1000 * 1000}, {"GB", 1000 * 1000 * 1000},
		{"TB", 1000 * 1000 * 1000 * 1000}, {"B", 1},
	}
	for _, unit := range units {
		if strings.HasSuffix(value, unit.name) {
			number := strings.TrimSpace(strings.TrimSuffix(value, unit.name))
			parsed, err := strconv.ParseFloat(number, 64)
			if err == nil {
				return int64(parsed * unit.factor)
			}
			return 0
		}
	}
	return 0
}
