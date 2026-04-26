package buildinfo

import "runtime/debug"

const (
	defaultVersion   = "dev"
	defaultCommitSHA = "unknown"
)

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
