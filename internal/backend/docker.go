package backend

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"slices"
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

const defaultDockerRunnerImage = "ghcr.io/quipper/actions-runner:2.337.0"

// DockerBackend runs each job inside an ephemeral Docker container.
type DockerBackend struct {
	image    string
	platform string
	runtime  string
	logger   *slog.Logger
}

// NewDockerBackend builds a Docker backend. An empty runtime uses the Docker
// default runtime. A named runtime such as runsc is passed to docker run.
func NewDockerBackend(image, platform, runtime string) *DockerBackend {
	if image == "" {
		image = defaultDockerRunnerImage
	}
	logger := slog.Default().With("image", image)
	if platform != "" {
		logger = logger.With("platform", platform)
	}
	if runtime != "" {
		logger = logger.With("runtime", runtime)
	}
	return &DockerBackend{
		image:    image,
		platform: platform,
		runtime:  runtime,
		logger:   logger,
	}
}

// Run starts a runner container and launches the GitHub Actions runner.
//
// The default image is quipper/actions-runner (github.com/quipper/actions-runner)
// whose entrypoint unconditionally starts dockerd, then execs CMD
// (/home/runner/run.sh) which reads ACTIONS_RUNNER_INPUT_JITCONFIG.
func (b *DockerBackend) Run(ctx context.Context, name, jitConfig string) error {
	ctx, span := dockerTracer.Start(ctx, "backend.docker.run",
		trace.WithAttributes(attribute.String("container.name", name)),
	)
	defer span.End()

	if err := b.checkRuntimeRegistered(ctx); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	cmd := exec.CommandContext(ctx, binpath.Lookup("docker"), b.runArgs(name, jitConfig)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		err = fmt.Errorf("docker run: %s: %w", string(out), err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	return nil
}

// runArgs builds the docker run arguments for one runner container.
// The default runtime keeps --privileged because Docker in Docker images need it.
// A named runtime such as runsc gets --runtime instead and runs without --privileged.
func (b *DockerBackend) runArgs(name, jitConfig string) []string {
	args := []string{"run", "-d"}
	if b.runtime == "" {
		args = append(args, "--privileged")
	} else {
		args = append(args, "--runtime="+b.runtime)
	}
	args = append(args,
		"--name", name,
		"-e", "ACTIONS_RUNNER_INPUT_JITCONFIG="+jitConfig,
	)
	if b.platform != "" {
		args = append(args, "--platform", b.platform)
	}
	return append(args, b.image)
}

// checkRuntimeRegistered fails fast when a named runtime is requested but
// Docker on this host does not list it.
func (b *DockerBackend) checkRuntimeRegistered(ctx context.Context) error {
	if b.runtime == "" {
		return nil
	}
	hostname, _ := os.Hostname()
	runtimes, err := DockerRuntimes(ctx)
	if err != nil {
		return fmt.Errorf("list Docker runtimes on host %s to check runtime %s: %w", hostname, b.runtime, err)
	}
	if !slices.Contains(runtimes, b.runtime) {
		return fmt.Errorf("runtime %s is not registered with Docker on host %s, known runtimes: %s", b.runtime, hostname, strings.Join(runtimes, ", "))
	}
	return nil
}

// DockerRuntimes returns the sorted names of the runtimes Docker on this host
// knows, for example runc and runsc.
func DockerRuntimes(ctx context.Context) ([]string, error) {
	cmd := exec.CommandContext(ctx, binpath.Lookup("docker"), "info", "--format", "{{json .Runtimes}}")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("docker info: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return parseDockerRuntimes(out)
}

// parseDockerRuntimes reads the runtime names from the JSON object that
// docker info prints for .Runtimes.
func parseDockerRuntimes(raw []byte) ([]string, error) {
	var byName map[string]json.RawMessage
	if err := json.Unmarshal(raw, &byName); err != nil {
		return nil, fmt.Errorf("parse Docker runtimes %q: %w", strings.TrimSpace(string(raw)), err)
	}
	names := make([]string, 0, len(byName))
	for name := range byName {
		names = append(names, name)
	}
	slices.Sort(names)
	return names, nil
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
