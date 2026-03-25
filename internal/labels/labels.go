package labels

import (
	"runtime"
	"strings"
)

// DetectPlatform returns the normalized OS and architecture from a platform
// string (e.g. "linux/arm64"). If platform is empty, it falls back to the
// current runtime.
func DetectPlatform(platform string) (os, arch string) {
	if platform != "" {
		parts := strings.SplitN(platform, "/", 2)
		if len(parts) == 2 {
			os = strings.ToLower(parts[0])
			arch = normalizeArch(parts[1])
			return os, arch
		}
	}
	return runtime.GOOS, normalizeArch(runtime.GOARCH)
}

// normalizeArch maps Go-style architecture names to GitHub runner conventions.
func normalizeArch(arch string) string {
	switch strings.ToLower(arch) {
	case "amd64", "x86_64", "x64":
		return "x64"
	case "arm64", "aarch64":
		return "arm64"
	default:
		return strings.ToLower(arch)
	}
}

// DefaultLabels returns the base labels for a given OS/arch combination,
// following GitHub-hosted runner naming conventions with lowercase.
func DefaultLabels(os, arch string) []string {
	osLabel := strings.ToLower(os)
	if osLabel == "darwin" {
		osLabel = "macos"
	}
	return []string{"self-hosted", osLabel, arch}
}

// DefaultVersionAliases returns the default version alias labels for a
// given OS/arch, mirroring GitHub-hosted runner label names.
func DefaultVersionAliases(os, arch string) []string {
	os = strings.ToLower(os)
	switch {
	case os == "linux" && arch == "x64":
		return []string{"ubuntu-latest", "ubuntu-24.04", "ubuntu-22.04"}
	case os == "linux" && arch == "arm64":
		return []string{"ubuntu-latest-arm", "ubuntu-24.04-arm", "ubuntu-22.04-arm"}
	case (os == "darwin" || os == "macos") && arch == "arm64":
		return []string{"macos-latest", "macos-15", "macos-14"}
	default:
		return nil
	}
}

// Option configures label resolution behavior.
type Option func(*resolveOptions)

type resolveOptions struct {
	versionAliases *[]string
}

// WithVersionAliases overrides the default version aliases. Pass an empty
// slice to disable version alias labels entirely.
func WithVersionAliases(aliases []string) Option {
	return func(o *resolveOptions) {
		o.versionAliases = &aliases
	}
}

// ResolveLabels computes the final label list for a runner scale set.
//
// If explicit is non-empty, it is used as-is (prepended with scaleSetName).
// Otherwise, labels are auto-generated from os/arch with version aliases and
// extra labels appended. Duplicates are removed while preserving order.
func ResolveLabels(scaleSetName string, explicit []string, extra []string, os, arch string, opts ...Option) []string {
	var o resolveOptions
	for _, fn := range opts {
		fn(&o)
	}

	result := []string{scaleSetName}

	if len(explicit) > 0 {
		result = append(result, explicit...)
		return result
	}

	result = append(result, DefaultLabels(os, arch)...)

	var aliases []string
	if o.versionAliases != nil {
		aliases = *o.versionAliases
	} else {
		aliases = DefaultVersionAliases(os, arch)
	}
	result = append(result, aliases...)

	result = appendDedup(result, extra)

	return result
}

// appendDedup appends items from extra to base, skipping any that already
// exist in base. Preserves order of base followed by new items from extra.
func appendDedup(base, extra []string) []string {
	seen := make(map[string]struct{}, len(base))
	for _, l := range base {
		seen[l] = struct{}{}
	}
	for _, l := range extra {
		if _, ok := seen[l]; !ok {
			base = append(base, l)
			seen[l] = struct{}{}
		}
	}
	return base
}
