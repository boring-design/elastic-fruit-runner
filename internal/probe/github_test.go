package probe

import (
	"strings"
	"testing"
)

func TestClassifyGitHubError(t *testing.T) {
	org := GitHubTarget{Org: "acme"}
	repo := GitHubTarget{Repo: "acme/app"}
	cases := []struct {
		name        string
		message     string
		target      GitHubTarget
		wantCheck   string
		wantContain string
	}{
		{"dns failure", "Get https://api.github.com: dial tcp: lookup api.github.com: no such host", org, checkGitHubReachable, "cannot reach api.github.com"},
		{"deadline", "failed to issue the request: context deadline exceeded", org, checkGitHubReachable, "cannot reach"},
		{"401 with dial text ignored", `request POST https://api.github.com/x failed(status="401 Unauthorized"): dial`, org, checkCredentialsValid, "401"},
		{"app token", "failed to fetch access token: bad key", org, checkCredentialsValid, "401"},
		{"403", `request POST https://api.github.com/x failed(status="403 Forbidden"): unexpected status code: 403`, org, checkRunnerPermission, "org acme (403)"},
		{"404 repo", `request POST https://api.github.com/x failed(status="404 Not Found"): unexpected`, repo, checkRunnerPermission, "repo acme/app was not found"},
		{"missing group", `request GET https://pipelines failed(status="200 OK"): no runner group found with name "team"`, org, checkRunnerGroup, "runner group team does not exist in org acme"},
		{"unknown", "something odd", org, checkRunnerPermission, "something odd"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotCheck, gotMessage := classifyGitHubError(tc.message, tc.target, "team")
			if gotCheck != tc.wantCheck {
				t.Fatalf("check = %q, want %q", gotCheck, tc.wantCheck)
			}
			if !strings.Contains(gotMessage, tc.wantContain) {
				t.Fatalf("message %q does not contain %q", gotMessage, tc.wantContain)
			}
		})
	}
}
