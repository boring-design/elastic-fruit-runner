package tart

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/boring-design/elastic-fruit-runner/internal/binpath"
)

func TestWaitForSSHUsesSSHTransport(t *testing.T) {
	dir := t.TempDir()
	argsFile := filepath.Join(dir, "args")
	writeFakeSSHPass(t, dir, `
printf '%s\n' "$@" > "`+argsFile+`"
exit 0
`)
	resetBinpath(t, dir)

	m := NewManager()
	if err := m.waitForSSH(context.Background(), "vm-1", "192.168.64.3"); err != nil {
		t.Fatalf("waitForSSH() error = %v", err)
	}

	argsBytes, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("read sshpass args: %v", err)
	}
	args := string(argsBytes)
	for _, want := range []string{
		"-p\nadmin\n",
		"ssh\n",
		"ConnectTimeout=5\n",
		"ConnectionAttempts=1\n",
		"admin@192.168.64.3\n",
		"true\n",
	} {
		if !strings.Contains(args, want) {
			t.Fatalf("sshpass args missing %q:\n%s", want, args)
		}
	}
}

func TestWaitForSSHReportsLastReadinessError(t *testing.T) {
	dir := t.TempDir()
	writeFakeSSHPass(t, dir, `
echo "connect: no route to host" >&2
exit 255
`)
	resetBinpath(t, dir)
	restore := setSSHReadyTimings(15*time.Millisecond, time.Millisecond, time.Millisecond, 10*time.Millisecond)
	t.Cleanup(restore)

	m := NewManager()
	err := m.waitForSSH(context.Background(), "vm-1", "192.168.64.3")
	if err == nil {
		t.Fatal("waitForSSH() error = nil, want failure")
	}
	for _, want := range []string{
		"SSH not reachable on vm-1 (192.168.64.3:22)",
		"last error",
		"connect: no route to host",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("waitForSSH() error missing %q:\n%v", want, err)
		}
	}
}

func TestImageExistsIncludesPulledOCIImages(t *testing.T) {
	dir := t.TempDir()
	argsFile := filepath.Join(dir, "tart.args")
	writeFakeTart(t, dir, `
printf '%s\n' "$@" > "`+argsFile+`"
if [ "$1" = "list" ] && [ "$2" = "--quiet" ]; then
  printf '%s\n' "ghcr.io/cirruslabs/macos-tahoe-xcode:latest"
  printf '%s\n' "local-template"
  exit 0
fi
exit 64
`)
	resetBinpath(t, dir)

	m := NewManager()
	exists, err := m.ImageExists(context.Background(), "ghcr.io/cirruslabs/macos-tahoe-xcode:latest")
	if err != nil {
		t.Fatalf("ImageExists() error = %v", err)
	}
	if !exists {
		t.Fatal("ImageExists() = false, want true for pulled OCI image")
	}

	argsBytes, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("read tart args: %v", err)
	}
	args := string(argsBytes)
	if strings.Contains(args, "--source") {
		t.Fatalf("ImageExists() should not restrict tart list source:\n%s", args)
	}
}

func writeFakeSSHPass(t *testing.T, dir, body string) {
	t.Helper()
	path := filepath.Join(dir, "sshpass")
	script := "#!/bin/sh\n" + body
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake sshpass: %v", err)
	}
}

func writeFakeTart(t *testing.T, dir, body string) {
	t.Helper()
	path := filepath.Join(dir, "tart")
	script := "#!/bin/sh\n" + body
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake tart: %v", err)
	}
}

func resetBinpath(t *testing.T, dir string) {
	t.Helper()
	t.Setenv("PATH", dir)
	oldDirs := binpath.ResetForTesting([]string{dir})
	t.Cleanup(func() {
		binpath.ResetForTesting(oldDirs)
	})
}

func setSSHReadyTimings(maxWait, initialBackoff, maxBackoff, attemptTimeout time.Duration) func() {
	oldMaxWait := sshReadyMaxWait
	oldInitialBackoff := sshReadyInitialBackoff
	oldMaxBackoff := sshReadyMaxBackoff
	oldAttemptTimeout := sshReadyAttemptTimeout

	sshReadyMaxWait = maxWait
	sshReadyInitialBackoff = initialBackoff
	sshReadyMaxBackoff = maxBackoff
	sshReadyAttemptTimeout = attemptTimeout

	return func() {
		sshReadyMaxWait = oldMaxWait
		sshReadyInitialBackoff = oldInitialBackoff
		sshReadyMaxBackoff = oldMaxBackoff
		sshReadyAttemptTimeout = oldAttemptTimeout
	}
}
