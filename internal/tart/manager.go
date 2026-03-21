package tart

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

var tracer = otel.Tracer("github.com/boring-design/elastic-fruit-runner/internal/tart")

// Manager wraps the tart CLI for VM lifecycle operations.
// All operations call `tart` which must be installed on the host.
type Manager struct {
	logger *slog.Logger
}

func NewManager(logger *slog.Logger) *Manager {
	return &Manager{logger: logger}
}

// Pull fetches a remote VM image (e.g. from a registry like ghcr.io).
func (m *Manager) Pull(ctx context.Context, image string) error {
	ctx, span := tracer.Start(ctx, "tart.pull",
		trace.WithAttributes(attribute.String("vm.image", image)),
	)
	defer span.End()

	m.logger.Info("pulling VM image", "image", image)
	if err := m.run(ctx, "pull", image); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	return nil
}

// ImageExists checks whether a VM image is available locally.
func (m *Manager) ImageExists(ctx context.Context, image string) (bool, error) {
	ctx, span := tracer.Start(ctx, "tart.image_exists",
		trace.WithAttributes(attribute.String("vm.image", image)),
	)
	defer span.End()

	cmd := exec.CommandContext(ctx, "tart", "list", "--source", "local", "--quiet")
	out, err := cmd.Output()
	if err != nil {
		span.RecordError(err)
		return false, fmt.Errorf("tart list: %w", err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if strings.TrimSpace(line) == image {
			return true, nil
		}
	}
	return false, nil
}

// Clone creates a new VM by cloning an existing image.
func (m *Manager) Clone(ctx context.Context, image, name string) error {
	ctx, span := tracer.Start(ctx, "tart.clone",
		trace.WithAttributes(
			attribute.String("vm.image", image),
			attribute.String("vm.name", name),
		),
	)
	defer span.End()

	m.logger.Info("cloning VM", "image", image, "name", name)
	if err := m.run(ctx, "clone", image, name); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	return nil
}

// Start launches a VM in the background (no graphics).
// Returns after the VM process has started; the VM runs asynchronously.
func (m *Manager) Start(ctx context.Context, name string) error {
	ctx, span := tracer.Start(ctx, "tart.start",
		trace.WithAttributes(attribute.String("vm.name", name)),
	)
	defer span.End()

	m.logger.Info("starting VM", "name", name)
	cmd := exec.CommandContext(ctx, "tart", "run", name, "--no-graphics")
	if err := cmd.Start(); err != nil {
		err = fmt.Errorf("start VM %s: %w", name, err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	// Detach: we don't wait for this process — it outlives this call.
	go func() { _ = cmd.Wait() }()
	return nil
}

// IPAddress waits up to 60 s for the VM to get a DHCP address and returns it.
func (m *Manager) IPAddress(ctx context.Context, name string) (string, error) {
	ctx, span := tracer.Start(ctx, "tart.ip_address",
		trace.WithAttributes(attribute.String("vm.name", name)),
	)
	defer span.End()

	m.logger.Info("waiting for VM IP", "name", name)
	cmd := exec.CommandContext(ctx, "tart", "ip", name, "--wait", "60")
	out, err := cmd.Output()
	if err != nil {
		err = fmt.Errorf("tart ip %s: %w", name, err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return "", err
	}
	ip := strings.TrimSpace(string(out))
	span.SetAttributes(attribute.String("vm.ip", ip))
	return ip, nil
}

// Exec runs a command inside the VM via SSH (using `tart ip` to discover the address).
// The default Cirrus Labs macOS base images use admin:admin credentials.
func (m *Manager) Exec(ctx context.Context, name string, args ...string) error {
	ctx, span := tracer.Start(ctx, "tart.ssh_exec",
		trace.WithAttributes(attribute.String("vm.name", name)),
	)
	defer span.End()

	ip, err := m.IPAddress(ctx, name)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	sshArgs := m.buildSSHArgs(ip, args...)
	m.logger.Info("ssh exec in VM", "name", name, "ip", ip, "args", args)
	var buf bytes.Buffer
	cmd := exec.CommandContext(ctx, "sshpass", sshArgs...)
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	if err := cmd.Run(); err != nil {
		err = fmt.Errorf("ssh exec %s (%s): %w\n%s", name, ip, err, buf.Bytes())
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	return nil
}

// Stop halts a running VM.
func (m *Manager) Stop(ctx context.Context, name string) error {
	ctx, span := tracer.Start(ctx, "tart.stop",
		trace.WithAttributes(attribute.String("vm.name", name)),
	)
	defer span.End()

	m.logger.Info("stopping VM", "name", name)
	if err := m.run(ctx, "stop", name); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	return nil
}

// Delete removes a stopped VM and its disk image.
func (m *Manager) Delete(ctx context.Context, name string) error {
	ctx, span := tracer.Start(ctx, "tart.delete",
		trace.WithAttributes(attribute.String("vm.name", name)),
	)
	defer span.End()

	m.logger.Info("deleting VM", "name", name)
	if err := m.run(ctx, "delete", name); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	return nil
}

// buildSSHArgs constructs sshpass + ssh arguments for executing a command in the VM.
// Uses admin:admin credentials (Cirrus Labs macOS base image default).
func (m *Manager) buildSSHArgs(ip string, args ...string) []string {
	sshArgs := []string{
		"-p", "admin",
		"ssh",
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "LogLevel=ERROR",
		"admin@" + ip,
	}
	return append(sshArgs, args...)
}

func (m *Manager) run(ctx context.Context, args ...string) error {
	var buf bytes.Buffer
	cmd := exec.CommandContext(ctx, "tart", args...)
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("tart %v: %w\n%s", args, err, buf.Bytes())
	}
	return nil
}
