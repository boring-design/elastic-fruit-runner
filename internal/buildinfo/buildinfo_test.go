package buildinfo_test

import (
	"runtime/debug"
	"testing"

	"github.com/boring-design/elastic-fruit-runner/internal/buildinfo"
)

func TestFromBuildInfoUsesMainVersionAndVCSRevision(t *testing.T) {
	t.Parallel()

	info := buildinfo.FromBuildInfo(&debug.BuildInfo{
		Main: debug.Module{
			Version: "v1.2.3",
		},
		Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: "0123456789abcdef"},
		},
	})

	if info.Version != "v1.2.3" {
		t.Fatalf("Version = %q, want %q", info.Version, "v1.2.3")
	}
	if info.CommitSHA != "0123456789abcdef" {
		t.Fatalf("CommitSHA = %q, want %q", info.CommitSHA, "0123456789abcdef")
	}
}

func TestFromBuildInfoFallsBackForDevelopmentBuilds(t *testing.T) {
	t.Parallel()

	info := buildinfo.FromBuildInfo(&debug.BuildInfo{
		Main: debug.Module{
			Version: "(devel)",
		},
	})

	if info.Version != "dev" {
		t.Fatalf("Version = %q, want %q", info.Version, "dev")
	}
	if info.CommitSHA != "unknown" {
		t.Fatalf("CommitSHA = %q, want %q", info.CommitSHA, "unknown")
	}
}
