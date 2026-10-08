package buildinfo

import "runtime/debug"

const (
	defaultVersion   = "dev"
	defaultCommitSHA = "unknown"
	defaultBuildDate = "unknown"
)

// Release builds set these values through ldflags, for example
// -X github.com/boring-design/elastic-fruit-runner/internal/buildinfo.version=1.2.3
// (see .goreleaser.yaml). A plain go build leaves them empty, and the
// functions below fall back to the Go build info.
var (
	version string
	commit  string
	date    string
)

// Version returns the version of the running binary.
// The CLI and the cloud report both use this value.
func Version() string {
	if version != "" {
		return version
	}
	return MainVersion(Current())
}

// Commit returns the Git commit of the running binary.
func Commit() string {
	if commit != "" {
		return commit
	}
	return VCSRevision(Current())
}

// Date returns the build date of the running binary.
func Date() string {
	if date != "" {
		return date
	}
	if vcsTime := Setting(Current(), "vcs.time"); vcsTime != "" {
		return vcsTime
	}
	return defaultBuildDate
}

// Current returns the Go build metadata embedded in the running binary.
func Current() *debug.BuildInfo {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return nil
	}
	return bi
}

// MainVersion returns the main module version from Go build info.
func MainVersion(bi *debug.BuildInfo) string {
	if bi == nil {
		return defaultVersion
	}
	if bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		return bi.Main.Version
	}
	return defaultVersion
}

// VCSRevision returns the Git revision from Go build settings.
func VCSRevision(bi *debug.BuildInfo) string {
	revision := Setting(bi, "vcs.revision")
	if revision == "" {
		return defaultCommitSHA
	}
	return revision
}

// Setting returns a single build setting value by key.
func Setting(bi *debug.BuildInfo, key string) string {
	if bi == nil {
		return ""
	}
	for _, setting := range bi.Settings {
		if setting.Key == key {
			return setting.Value
		}
	}
	return ""
}
