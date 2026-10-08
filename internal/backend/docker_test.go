package backend

import (
	"slices"
	"testing"
)

func TestDockerRunArgsDefaultRuntimeIsPrivileged(t *testing.T) {
	t.Parallel()
	b := NewDockerBackend("ghcr.io/example/runner:1", "linux/arm64", "")
	got := b.runArgs("efr-1", "jit")
	want := []string{
		"run", "-d", "--privileged",
		"--name", "efr-1",
		"-e", "ACTIONS_RUNNER_INPUT_JITCONFIG=jit",
		"--platform", "linux/arm64",
		"ghcr.io/example/runner:1",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("runArgs = %q, want %q", got, want)
	}
}

func TestDockerRunArgsNamedRuntimeDropsPrivileged(t *testing.T) {
	t.Parallel()
	b := NewDockerBackend("ghcr.io/example/runner:1", "", "runsc")
	got := b.runArgs("efr-2", "jit")
	want := []string{
		"run", "-d", "--runtime=runsc",
		"--name", "efr-2",
		"-e", "ACTIONS_RUNNER_INPUT_JITCONFIG=jit",
		"ghcr.io/example/runner:1",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("runArgs = %q, want %q", got, want)
	}
	if slices.Contains(got, "--privileged") {
		t.Fatal("runArgs with a named runtime must not contain --privileged")
	}
}

func TestParseDockerRuntimes(t *testing.T) {
	t.Parallel()
	raw := []byte(`{"runsc":{"path":"/usr/bin/runsc"},"io.containerd.runc.v2":{"path":"runc"},"runc":{"path":"runc"}}`)
	got, err := parseDockerRuntimes(raw)
	if err != nil {
		t.Fatalf("parseDockerRuntimes: %v", err)
	}
	want := []string{"io.containerd.runc.v2", "runc", "runsc"}
	if !slices.Equal(got, want) {
		t.Fatalf("parseDockerRuntimes = %q, want %q", got, want)
	}
	if _, err := parseDockerRuntimes([]byte("not json")); err == nil {
		t.Fatal("parseDockerRuntimes must fail on invalid JSON")
	}
}

func TestNewRejectsRuntimeForTart(t *testing.T) {
	t.Parallel()
	if _, err := New(Spec{Backend: "tart", Runtime: "runsc"}); err == nil {
		t.Fatal("New must reject a runtime for the tart backend")
	}
	if _, err := New(Spec{Backend: "docker", Runtime: "runsc"}); err != nil {
		t.Fatalf("New docker with runtime: %v", err)
	}
}
