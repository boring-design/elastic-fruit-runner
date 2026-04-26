package tart

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/boring-design/elastic-fruit-runner/internal/binpath"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

var tracer = otel.Tracer("github.com/boring-design/elastic-fruit-runner/internal/tart")

const ipAddressWaitSeconds = "180"

var (
	sshReadyMaxWait        = 120 * time.Second
	sshReadyInitialBackoff = 1 * time.Second
	sshReadyMaxBackoff     = 16 * time.Second
	sshReadyAttemptTimeout = 5 * time.Second
)

// Manager wraps the tart CLI for VM lifecycle operations.
// All operations call `tart` which must be installed on the host.
type Manager struct{}

func NewManager() *Manager {
	return &Manager{}
}

// List returns the names of all local VMs.
func (m *Manager) List(ctx context.Context) ([]string, error) {
	cmd := exec.CommandContext(ctx, binpath.Lookup("tart"), "list", "--source", "local", "--quiet")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("tart list: %w", err)
	}
	var names []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			names = append(names, line)
		}
	}
	return names, nil
}

// Pull fetches a remote VM image (e.g. from a registry like ghcr.io).
func (m *Manager) Pull(ctx context.Context, image string) error {
	ctx, span := tracer.Start(ctx, "tart.pull",
		trace.WithAttributes(attribute.String("vm.image", image)),
	)
	defer span.End()

	slog.Info("pulling VM image", "image", image)
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

	cmd := exec.CommandContext(ctx, binpath.Lookup("tart"), "list", "--source", "local", "--quiet")
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

	slog.Info("cloning VM", "image", image, "name", name)
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

	slog.Info("starting VM", "name", name)
	cmd := exec.CommandContext(ctx, binpath.Lookup("tart"), "run", name, "--no-graphics")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
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

// IPAddress waits for the VM to get a DHCP address and returns it.
func (m *Manager) IPAddress(ctx context.Context, name string) (string, error) {
	ctx, span := tracer.Start(ctx, "tart.ip_address",
		trace.WithAttributes(attribute.String("vm.name", name)),
	)
	defer span.End()

	slog.Info("waiting for VM IP", "name", name)
	cmd := exec.CommandContext(ctx, binpath.Lookup("tart"), "ip", name, "--wait", ipAddressWaitSeconds)
	out, err := cmd.CombinedOutput()
	if err != nil {
		err = fmt.Errorf("tart ip %s: %w\n%s", name, err, out)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return "", err
	}
	ip := strings.TrimSpace(string(out))
	span.SetAttributes(attribute.String("vm.ip", ip))
	return ip, nil
}

// Exec runs a command inside the VM via SSH (using `tart ip` to discover the address).
// The default Cirrus Labs macos base images use admin:admin credentials.
// It waits for SSH to become reachable with exponential backoff before executing.
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

	if err := m.waitForSSH(ctx, name, ip); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	sshArgs := m.buildSSHArgs(ip, args...)
	slog.Info("ssh exec in VM", "name", name, "ip", ip, "args", args)
	var buf bytes.Buffer
	cmd := exec.CommandContext(ctx, binpath.Lookup("sshpass"), sshArgs...)
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

// waitForSSH verifies the actual SSH transport used by Exec instead of only
// probing TCP port 22. On macOS launchd services, raw Go TCP dials can fail
// against Tart bridge addresses even when ssh can connect successfully.
func (m *Manager) waitForSSH(ctx context.Context, name, ip string) error {
	deadline := time.Now().Add(sshReadyMaxWait)
	backoff := sshReadyInitialBackoff
	var lastErr error

	for {
		err := m.probeSSH(ctx, name, ip)
		if err == nil {
			return nil
		}
		lastErr = err

		if time.Now().After(deadline) {
			return fmt.Errorf("SSH not reachable on %s (%s:22) after %s: last error: %w", name, ip, sshReadyMaxWait, lastErr)
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}

		slog.Info("waiting for SSH", "name", name, "ip", ip, "retry_in", backoff, "err", lastErr)
		select {
		case <-time.After(backoff):
		case <-ctx.Done():
			return ctx.Err()
		}
		backoff = min(backoff*2, sshReadyMaxBackoff)
	}
}

func (m *Manager) probeSSH(ctx context.Context, name, ip string) error {
	attemptCtx, cancel := context.WithTimeout(ctx, sshReadyAttemptTimeout)
	defer cancel()

	var buf bytes.Buffer
	cmd := exec.CommandContext(attemptCtx, binpath.Lookup("sshpass"), m.buildSSHArgs(ip, "true")...)
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	if err := cmd.Run(); err != nil {
		output := strings.TrimSpace(buf.String())
		if output != "" {
			return fmt.Errorf("ssh readiness probe %s (%s): %w: %s", name, ip, err, output)
		}
		return fmt.Errorf("ssh readiness probe %s (%s): %w", name, ip, err)
	}
	return nil
}

// Stop halts a running VM.
func (m *Manager) Stop(ctx context.Context, name string) error {
	ctx, span := tracer.Start(ctx, "tart.stop",
		trace.WithAttributes(attribute.String("vm.name", name)),
	)
	defer span.End()

	slog.Info("stopping VM", "name", name)
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

	slog.Info("deleting VM", "name", name)
	if err := m.run(ctx, "delete", name); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	return nil
}

// buildSSHArgs constructs sshpass + ssh arguments for executing a command in the VM.
// Uses admin:admin credentials (Cirrus Labs macos base image default).
func (m *Manager) buildSSHArgs(ip string, args ...string) []string {
	sshArgs := []string{
		"-p", "admin",
		"ssh",
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "LogLevel=ERROR",
		"-o", "ConnectTimeout=5",
		"-o", "ConnectionAttempts=1",
		"admin@" + ip,
	}
	return append(sshArgs, args...)
}

func (m *Manager) run(ctx context.Context, args ...string) error {
	var buf bytes.Buffer
	cmd := exec.CommandContext(ctx, binpath.Lookup("tart"), args...)
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("tart %v: %w\n%s", args, err, buf.Bytes())
	}
	return nil
}
