package config

import (
	"strings"
	"testing"
)

func TestWithCloudBlockCreatesMinimalConfig(t *testing.T) {
	t.Parallel()
	out, err := WithCloudBlock(nil, &CloudConfig{ServerURL: "https://cloud.example.com", MaxRunners: 2})
	if err != nil {
		t.Fatalf("WithCloudBlock() error: %v", err)
	}
	result := ValidateYAML(out)
	if len(result.Errors) > 0 {
		t.Fatalf("generated config is not valid: %s\n%s", result.Errors[0], out)
	}
	if result.Config.Mode() != RunModeCloud {
		t.Fatalf("mode = %s, want %s", result.Config.Mode(), RunModeCloud)
	}
	if result.Config.Cloud.MaxRunners != 2 {
		t.Fatalf("max_runners = %d, want 2", result.Config.Cloud.MaxRunners)
	}
}

func TestWithCloudBlockKeepsOtherKeysAndReplacesCloud(t *testing.T) {
	t.Parallel()
	existing := []byte("# keep me\nlog_level: debug\ncloud:\n  server_url: https://old.example.com\n  max_runners: 1\n")
	out, err := WithCloudBlock(existing, &CloudConfig{ServerURL: "https://new.example.com", MaxRunners: 3})
	if err != nil {
		t.Fatalf("WithCloudBlock() error: %v", err)
	}
	text := string(out)
	for _, want := range []string{"# keep me", "log_level: debug", "https://new.example.com", "max_runners: 3"} {
		if !strings.Contains(text, want) {
			t.Errorf("output is missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "old.example.com") {
		t.Errorf("output still has the old server url:\n%s", text)
	}
}

func TestWithCloudBlockRejectsGitHubTargets(t *testing.T) {
	t.Parallel()
	existing := []byte("orgs:\n  - org: acme\n")
	_, err := WithCloudBlock(existing, &CloudConfig{ServerURL: "https://cloud.example.com", MaxRunners: 1})
	if err == nil || !strings.Contains(err.Error(), `"orgs"`) {
		t.Fatalf("WithCloudBlock() error = %v, want it to name the orgs block", err)
	}
}
