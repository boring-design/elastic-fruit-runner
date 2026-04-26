package buildinfo_test

import (
	"runtime/debug"
	"testing"

	"github.com/boring-design/elastic-fruit-runner/internal/buildinfo"
)

func TestMainVersionUsesBuildInfoMainVersion(t *testing.T) {
	t.Parallel()

	bi := &debug.BuildInfo{
		Main: debug.Module{
			Version: "v1.2.3",
		},
		Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: "0123456789abcdef"},
		},
	}

	if got := buildinfo.MainVersion(bi); got != "v1.2.3" {
		t.Fatalf("MainVersion() = %q, want %q", got, "v1.2.3")
	}
	if got := buildinfo.VCSRevision(bi); got != "0123456789abcdef" {
		t.Fatalf("VCSRevision() = %q, want %q", got, "0123456789abcdef")
	}
}

func TestMainVersionFallsBackForDevelopmentBuilds(t *testing.T) {
	t.Parallel()

	bi := &debug.BuildInfo{
		Main: debug.Module{
			Version: "(devel)",
		},
	}

	if got := buildinfo.MainVersion(bi); got != "dev" {
		t.Fatalf("MainVersion() = %q, want %q", got, "dev")
	}
	if got := buildinfo.VCSRevision(bi); got != "unknown" {
		t.Fatalf("VCSRevision() = %q, want %q", got, "unknown")
	}
}
