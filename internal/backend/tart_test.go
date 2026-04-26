package backend

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boring-design/elastic-fruit-runner/internal/binpath"
)

func TestTartBackendCleanupRunsTartByDefault(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "tart-args.log")
	writeFakeTart(t, dir, `printf '%s\n' "$@" >> "`+logFile+`"
exit 0
`)
	resetBinpath(t, dir)

	b := NewTartBackend("ghcr.io/cirruslabs/macos-tahoe-base:latest")
	b.Cleanup(context.Background(), "vm-default-clean")

	contents, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("read fake tart log: %v", err)
	}
	got := string(contents)
	for _, want := range []string{"stop\n", "delete\n", "vm-default-clean\n"} {
		if !strings.Contains(got, want) {
			t.Fatalf("fake tart log missing %q:\n%s", want, got)
		}
	}
}

func TestTartBackendPreservesFailedVMsWhenEnvSet(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "tart-args.log")
	writeFakeTart(t, dir, `printf '%s\n' "$@" >> "`+logFile+`"
exit 0
`)
	resetBinpath(t, dir)
	t.Setenv(envPreserveFailedVMs, "true")

	b := NewTartBackend("ghcr.io/cirruslabs/macos-tahoe-base:latest")
	b.markVMFailed("vm-preserve-me")
	b.Cleanup(context.Background(), "vm-preserve-me")

	if data, err := os.ReadFile(logFile); err == nil && len(data) > 0 {
		t.Fatalf("fake tart should not have been invoked when preserving failed VM, got:\n%s", data)
	}
}

func TestTartBackendCleansSuccessfulVMsEvenWithPreserveFlag(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "tart-args.log")
	writeFakeTart(t, dir, `printf '%s\n' "$@" >> "`+logFile+`"
exit 0
`)
	resetBinpath(t, dir)
	t.Setenv(envPreserveFailedVMs, "true")

	b := NewTartBackend("ghcr.io/cirruslabs/macos-tahoe-base:latest")
	b.Cleanup(context.Background(), "vm-job-completed")

	contents, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("read fake tart log: %v", err)
	}
	got := string(contents)
	for _, want := range []string{"stop\n", "delete\n", "vm-job-completed\n"} {
		if !strings.Contains(got, want) {
			t.Fatalf("fake tart log missing %q:\n%s", want, got)
		}
	}
}

func TestTartBackendIgnoresPreserveFlagWhenEnvUnset(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "tart-args.log")
	writeFakeTart(t, dir, `printf '%s\n' "$@" >> "`+logFile+`"
exit 0
`)
	resetBinpath(t, dir)
	os.Unsetenv(envPreserveFailedVMs)

	b := NewTartBackend("ghcr.io/cirruslabs/macos-tahoe-base:latest")
	b.markVMFailed("vm-no-preserve")
	b.Cleanup(context.Background(), "vm-no-preserve")

	contents, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("read fake tart log: %v", err)
	}
	got := string(contents)
	if !strings.Contains(got, "delete\n") {
		t.Fatalf("expected fake tart delete invocation when preserve env is unset, got:\n%s", got)
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
