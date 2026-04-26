package controller

import "testing"

func TestRepositoryFromJobMessage(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		owner string
		repo  string
		want  string
	}{
		{name: "owner_and_repo", owner: "boring-design", repo: "elastic-fruit-runner", want: "boring-design/elastic-fruit-runner"},
		{name: "missing_owner", owner: "", repo: "elastic-fruit-runner", want: ""},
		{name: "missing_repo", owner: "boring-design", repo: "", want: ""},
		{name: "trims_whitespace", owner: " owner ", repo: " repo ", want: "owner/repo"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := repositoryFromJobMessage(tc.owner, tc.repo)
			if got != tc.want {
				t.Errorf("repositoryFromJobMessage(%q, %q) = %q, want %q", tc.owner, tc.repo, got, tc.want)
			}
		})
	}
}

func TestWorkflowNameFromJobMessage(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		displayName string
		workflowRef string
		want        string
	}{
		{name: "uses_display_name", displayName: "Unit Test", workflowRef: "owner/repo/.github/workflows/ci.yml@refs/heads/main", want: "Unit Test"},
		{name: "falls_back_to_basename", displayName: "", workflowRef: "owner/repo/.github/workflows/ci.yml@refs/heads/main", want: "ci.yml"},
		{name: "ref_without_at", displayName: "", workflowRef: "owner/repo/.github/workflows/release.yml", want: "release.yml"},
		{name: "ref_without_slash", displayName: "", workflowRef: "ci.yml", want: "ci.yml"},
		{name: "all_empty", displayName: "", workflowRef: "", want: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := workflowNameFromJobMessage(tc.displayName, tc.workflowRef)
			if got != tc.want {
				t.Errorf("workflowNameFromJobMessage(%q, %q) = %q, want %q", tc.displayName, tc.workflowRef, got, tc.want)
			}
		})
	}
}

func TestWorkflowRunIDFromJobMessage(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   int64
		want string
	}{
		{name: "zero_returns_empty", in: 0, want: ""},
		{name: "positive_id", in: 1234567890, want: "1234567890"},
		{name: "negative_id", in: -1, want: "-1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := workflowRunIDFromJobMessage(tc.in)
			if got != tc.want {
				t.Errorf("workflowRunIDFromJobMessage(%d) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
