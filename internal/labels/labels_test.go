package labels

import (
	"testing"
)

func TestDetectPlatform_FromExplicitPlatformField(t *testing.T) {
	tests := []struct {
		name     string
		platform string
		wantOS   string
		wantArch string
	}{
		{"linux/amd64", "linux/amd64", "linux", "x64"},
		{"linux/arm64", "linux/arm64", "linux", "arm64"},
		{"linux/x64 alias", "linux/x64", "linux", "x64"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os, arch := DetectPlatform(tt.platform)
			if os != tt.wantOS {
				t.Errorf("DetectPlatform(%q) os = %q, want %q", tt.platform, os, tt.wantOS)
			}
			if arch != tt.wantArch {
				t.Errorf("DetectPlatform(%q) arch = %q, want %q", tt.platform, arch, tt.wantArch)
			}
		})
	}
}

func TestDetectPlatform_EmptyFallsBackToRuntime(t *testing.T) {
	os, arch := DetectPlatform("")
	if os == "" || arch == "" {
		t.Errorf("DetectPlatform(\"\") returned empty os=%q or arch=%q", os, arch)
	}
}

func TestDefaultLabels(t *testing.T) {
	tests := []struct {
		name     string
		os       string
		arch     string
		expected []string
	}{
		{
			"linux x64",
			"linux", "x64",
			[]string{"self-hosted", "linux", "x64"},
		},
		{
			"linux arm64",
			"linux", "arm64",
			[]string{"self-hosted", "linux", "arm64"},
		},
		{
			"macos arm64",
			"darwin", "arm64",
			[]string{"self-hosted", "macos", "arm64"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DefaultLabels(tt.os, tt.arch)
			if len(got) != len(tt.expected) {
				t.Fatalf("DefaultLabels(%q, %q) = %v, want %v", tt.os, tt.arch, got, tt.expected)
			}
			for i, l := range got {
				if l != tt.expected[i] {
					t.Errorf("DefaultLabels(%q, %q)[%d] = %q, want %q", tt.os, tt.arch, i, l, tt.expected[i])
				}
			}
		})
	}
}

func TestDefaultVersionAliases(t *testing.T) {
	tests := []struct {
		name     string
		os       string
		arch     string
		expected []string
	}{
		{
			"linux x64",
			"linux", "x64",
			[]string{"ubuntu-latest", "ubuntu-24.04", "ubuntu-22.04"},
		},
		{
			"linux arm64",
			"linux", "arm64",
			[]string{"ubuntu-latest-arm", "ubuntu-24.04-arm", "ubuntu-22.04-arm"},
		},
		{
			"macos arm64",
			"darwin", "arm64",
			[]string{"macos-latest", "macos-15", "macos-14"},
		},
		{
			"unknown platform",
			"freebsd", "amd64",
			nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DefaultVersionAliases(tt.os, tt.arch)
			if len(got) != len(tt.expected) {
				t.Fatalf("DefaultVersionAliases(%q, %q) = %v, want %v", tt.os, tt.arch, got, tt.expected)
			}
			for i, l := range got {
				if l != tt.expected[i] {
					t.Errorf("DefaultVersionAliases(%q, %q)[%d] = %q, want %q", tt.os, tt.arch, i, l, tt.expected[i])
				}
			}
		})
	}
}

func TestResolveLabels_ExplicitOverride(t *testing.T) {
	got := ResolveLabels("my-set", []string{"custom-a", "custom-b"}, nil, "", "")
	expected := []string{"my-set", "custom-a", "custom-b"}
	if len(got) != len(expected) {
		t.Fatalf("ResolveLabels explicit = %v, want %v", got, expected)
	}
	for i, l := range got {
		if l != expected[i] {
			t.Errorf("ResolveLabels explicit[%d] = %q, want %q", i, l, expected[i])
		}
	}
}

func TestResolveLabels_AutoDetectWithVersionAliases(t *testing.T) {
	got := ResolveLabels("my-set", nil, nil, "linux", "x64")
	expected := []string{
		"my-set",
		"self-hosted", "linux", "x64",
		"ubuntu-latest", "ubuntu-24.04", "ubuntu-22.04",
	}
	if len(got) != len(expected) {
		t.Fatalf("ResolveLabels auto = %v, want %v", got, expected)
	}
	for i, l := range got {
		if l != expected[i] {
			t.Errorf("ResolveLabels auto[%d] = %q, want %q", i, l, expected[i])
		}
	}
}

func TestResolveLabels_AutoDetectWithExtraLabels(t *testing.T) {
	got := ResolveLabels("my-set", nil, []string{"gpu", "custom"}, "linux", "x64")
	expected := []string{
		"my-set",
		"self-hosted", "linux", "x64",
		"ubuntu-latest", "ubuntu-24.04", "ubuntu-22.04",
		"gpu", "custom",
	}
	if len(got) != len(expected) {
		t.Fatalf("ResolveLabels extra = %v, want %v", got, expected)
	}
	for i, l := range got {
		if l != expected[i] {
			t.Errorf("ResolveLabels extra[%d] = %q, want %q", i, l, expected[i])
		}
	}
}

func TestResolveLabels_ExplicitVersionAliases(t *testing.T) {
	got := ResolveLabels("my-set", nil, nil, "linux", "x64",
		WithVersionAliases([]string{"ubuntu-24.04"}))
	expected := []string{
		"my-set",
		"self-hosted", "linux", "x64",
		"ubuntu-24.04",
	}
	if len(got) != len(expected) {
		t.Fatalf("ResolveLabels version aliases = %v, want %v", got, expected)
	}
	for i, l := range got {
		if l != expected[i] {
			t.Errorf("ResolveLabels version aliases[%d] = %q, want %q", i, l, expected[i])
		}
	}
}

func TestResolveLabels_EmptyVersionAliasesDisablesDefaults(t *testing.T) {
	got := ResolveLabels("my-set", nil, nil, "linux", "x64",
		WithVersionAliases([]string{}))
	expected := []string{
		"my-set",
		"self-hosted", "linux", "x64",
	}
	if len(got) != len(expected) {
		t.Fatalf("ResolveLabels no aliases = %v, want %v", got, expected)
	}
	for i, l := range got {
		if l != expected[i] {
			t.Errorf("ResolveLabels no aliases[%d] = %q, want %q", i, l, expected[i])
		}
	}
}

func TestResolveLabels_DeduplicatesLabels(t *testing.T) {
	got := ResolveLabels("my-set", nil, []string{"self-hosted", "linux", "custom"}, "linux", "x64",
		WithVersionAliases([]string{}))
	expected := []string{
		"my-set",
		"self-hosted", "linux", "x64",
		"custom",
	}
	if len(got) != len(expected) {
		t.Fatalf("ResolveLabels dedup = %v, want %v", got, expected)
	}
	for i, l := range got {
		if l != expected[i] {
			t.Errorf("ResolveLabels dedup[%d] = %q, want %q", i, l, expected[i])
		}
	}
}
