package buildinfo

import "runtime/debug"

const (
	defaultVersion   = "dev"
	defaultCommitSHA = "unknown"
)

// Info contains the build metadata surfaced by Go's embedded build info.
type Info struct {
	Version   string
	CommitSHA string
}

// Current returns build metadata for the running binary.
func Current() Info {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return Info{
			Version:   defaultVersion,
			CommitSHA: defaultCommitSHA,
		}
	}
	return FromBuildInfo(bi)
}

// FromBuildInfo converts Go's standard build info into the metadata exposed by
// the controller and API.
func FromBuildInfo(bi *debug.BuildInfo) Info {
	info := Info{
		Version:   defaultVersion,
		CommitSHA: defaultCommitSHA,
	}
	if bi == nil {
		return info
	}

	if bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		info.Version = bi.Main.Version
	}
	for _, setting := range bi.Settings {
		if setting.Key == "vcs.revision" && setting.Value != "" {
			info.CommitSHA = setting.Value
			break
		}
	}

	return info
}
