package tart

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"sync"
)

const (
	// Default SSH credentials for Cirrus Labs macOS images.
	sshUser     = "admin"
	sshPassword = "admin"
)

// Manager wraps the tart CLI for VM lifecycle operations.
// All operations call `tart` which must be installed on the host.
// Commands inside VMs are executed via SSH (tart exec requires newer tart versions).
type Manager struct {
	logger *slog.Logger

	mu  sync.Mutex
	ips map[string]string
}

func NewManager(logger *slog.Logger) *Manager {
	return &Manager{
		logger: logger,
		ips:    make(map[string]string),
	}
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
// The resolved IP is cached for subsequent SSH calls.
func (m *Manager) IPAddress(ctx context.Context, name string) (string, error) {
	m.logger.Info("waiting for VM IP", "name", name)
	cmd := exec.CommandContext(ctx, "tart", "ip", name, "--wait", "60")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("tart ip %s: %w", name, err)
	}
	ip := strings.TrimSpace(string(out))
	m.mu.Lock()
	m.ips[name] = ip
	m.mu.Unlock()
	return ip, nil
}

// ExecOutput runs a command inside the VM via SSH and returns combined stdout+stderr.
func (m *Manager) ExecOutput(ctx context.Context, name string, args ...string) ([]byte, error) {
	m.mu.Lock()
	ip := m.ips[name]
	m.mu.Unlock()
	if ip == "" {
		return nil, fmt.Errorf("no IP cached for VM %s; call IPAddress first", name)
	}

	quoted := make([]string, len(args))
	for i, a := range args {
		quoted[i] = shellQuote(a)
	}
	remoteCmd := strings.Join(quoted, " ")
	m.logger.Info("ssh exec in VM", "name", name, "ip", ip, "cmd", remoteCmd)

	sshArgs := []string{
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "LogLevel=ERROR",
		fmt.Sprintf("%s@%s", sshUser, ip),
		remoteCmd,
	}

	var buf bytes.Buffer
	cmd := exec.CommandContext(ctx, "sshpass", append([]string{"-p", sshPassword, "ssh"}, sshArgs...)...)
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	if err := cmd.Run(); err != nil {
		return buf.Bytes(), fmt.Errorf("ssh exec %s (%s): %w\n%s", name, ip, err, buf.Bytes())
	}
	return buf.Bytes(), nil
}

// Stop halts a running VM.
func (m *Manager) Stop(ctx context.Context, name string) error {
	m.logger.Info("stopping VM", "name", name)
	m.mu.Lock()
	delete(m.ips, name)
	m.mu.Unlock()
	return m.run(ctx, "stop", name)
}

// Delete removes a stopped VM and its disk image.
func (m *Manager) Delete(ctx context.Context, name string) error {
	m.logger.Info("deleting VM", "name", name)
	return m.run(ctx, "delete", name)
}

// shellQuote wraps s in single quotes, escaping any embedded single quotes.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
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
