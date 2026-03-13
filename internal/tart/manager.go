package tart

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
)

// Manager wraps the tart CLI for VM lifecycle operations.
// All operations call `tart` which must be installed on the host.
type Manager struct {
	logger *slog.Logger
}

func NewManager(logger *slog.Logger) *Manager {
	return &Manager{logger: logger}
}

// Clone creates a new VM by cloning an existing image.
func (m *Manager) Clone(ctx context.Context, image, name string) error {
	m.logger.Info("cloning VM", "image", image, "name", name)
	return m.run(ctx, "clone", image, name)
}

// Start launches a VM in the background (no graphics).
// Returns after the VM process has started; the VM runs asynchronously.
func (m *Manager) Start(ctx context.Context, name string) error {
	m.logger.Info("starting VM", "name", name)
	cmd := exec.CommandContext(ctx, "tart", "run", name, "--no-graphics")
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start VM %s: %w", name, err)
	}
	// Detach: we don't wait for this process — it outlives this call.
	go func() { _ = cmd.Wait() }()
	return nil
}

// IPAddress waits up to 60 s for the VM to get a DHCP address and returns it.
func (m *Manager) IPAddress(ctx context.Context, name string) (string, error) {
	m.logger.Info("waiting for VM IP", "name", name)
	cmd := exec.CommandContext(ctx, "tart", "ip", name, "--wait", "60")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("tart ip %s: %w", name, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// Exec runs a command inside the VM via `tart exec`.
func (m *Manager) Exec(ctx context.Context, name string, args ...string) error {
	cmdArgs := append([]string{"exec", name, "--"}, args...)
	m.logger.Info("exec in VM", "name", name, "args", args)
	return m.run(ctx, cmdArgs...)
}

// ExecOutput runs a command inside the VM and returns combined stdout+stderr.
func (m *Manager) ExecOutput(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmdArgs := append([]string{"exec", name, "--"}, args...)
	var buf bytes.Buffer
	cmd := exec.CommandContext(ctx, "tart", cmdArgs...)
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	if err := cmd.Run(); err != nil {
		return buf.Bytes(), fmt.Errorf("tart exec %s %v: %w\n%s", name, args, err, buf.Bytes())
	}
	return buf.Bytes(), nil
}

// Stop halts a running VM.
func (m *Manager) Stop(ctx context.Context, name string) error {
	m.logger.Info("stopping VM", "name", name)
	return m.run(ctx, "stop", name)
}

// Delete removes a stopped VM and its disk image.
func (m *Manager) Delete(ctx context.Context, name string) error {
	m.logger.Info("deleting VM", "name", name)
	return m.run(ctx, "delete", name)
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
