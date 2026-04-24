package controller

import (
	"runtime/debug"
	"testing"
)

func TestApplyBuildInfoFallbackUsesModuleVersion(t *testing.T) {
	originalVersion := Version
	originalCommitSHA := CommitSHA
	defer func() {
		Version = originalVersion
		CommitSHA = originalCommitSHA
	}()

	Version = "dev"
	CommitSHA = "unknown"

	applyBuildInfoFallback(&debug.BuildInfo{
		Main: debug.Module{Version: "v1.2.3"},
	})

	if Version != "v1.2.3" {
		t.Fatalf("Version = %q, want %q", Version, "v1.2.3")
	}
}

func TestApplyBuildInfoFallbackUsesShortVCSRevision(t *testing.T) {
	originalVersion := Version
	originalCommitSHA := CommitSHA
	defer func() {
		Version = originalVersion
		CommitSHA = originalCommitSHA
	}()

	Version = "dev"
	CommitSHA = "unknown"

	applyBuildInfoFallback(&debug.BuildInfo{
		Main: debug.Module{Version: "(devel)"},
		Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: "1234567890abcdef"},
		},
	})

	if Version != "dev" {
		t.Fatalf("Version = %q, want %q", Version, "dev")
	}
	if CommitSHA != "1234567" {
		t.Fatalf("CommitSHA = %q, want %q", CommitSHA, "1234567")
	}
}

func TestApplyBuildInfoFallbackPreservesInjectedValues(t *testing.T) {
	originalVersion := Version
	originalCommitSHA := CommitSHA
	defer func() {
		Version = originalVersion
		CommitSHA = originalCommitSHA
	}()

	Version = "v9.9.9"
	CommitSHA = "abcdef0"

	applyBuildInfoFallback(&debug.BuildInfo{
		Main: debug.Module{Version: "v1.2.3"},
		Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: "1234567890abcdef"},
		},
	})

	if Version != "v9.9.9" {
		t.Fatalf("Version = %q, want %q", Version, "v9.9.9")
	}
	if CommitSHA != "abcdef0" {
		t.Fatalf("CommitSHA = %q, want %q", CommitSHA, "abcdef0")
	}
}
