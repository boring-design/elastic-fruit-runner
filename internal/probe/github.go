package probe

import (
	"context"
	"encoding/pem"
	"os"
	"strings"
	"time"

	"github.com/actions/scaleset"
	"github.com/hashicorp/go-retryablehttp"

	"github.com/boring-design/elastic-fruit-runner/config"
	"github.com/boring-design/elastic-fruit-runner/internal/githubclient"
)

// GitHubTarget is one org or repo to check. Exactly one of Org or Repo is set.
type GitHubTarget struct {
	Org         string
	Repo        string
	RunnerGroup string
}

// Name returns the target as shown to the user.
func (t GitHubTarget) Name() string {
	if t.Org != "" {
		return "org " + t.Org
	}
	return "repo " + t.Repo
}

func (t GitHubTarget) configURL() string {
	if t.Org != "" {
		return "https://github.com/" + t.Org
	}
	return "https://github.com/" + t.Repo
}

const githubTimeout = 20 * time.Second

const (
	checkPrivateKey       = "private_key"
	checkGitHubReachable  = "github_reachable"
	checkCredentialsValid = "credentials_valid"
	checkRunnerPermission = "runner_permission"
	checkRunnerGroup      = "runner_group"
)

// TestGitHubAuth proves that the credentials can manage runners for the target.
// One scale set call covers the whole chain: registration token, runner
// registration, then a runner group lookup.
func TestGitHubAuth(ctx context.Context, target GitHubTarget, auth *config.AuthConfig) []Check {
	ctx, cancel := context.WithTimeout(ctx, githubTimeout)
	defer cancel()

	var checks []Check
	if auth.Mode() == config.AuthModeGitHubApp {
		if message := checkPrivateKeyFile(auth.GitHubApp.PrivateKeyPath); message != "" {
			checks = append(checks, Check{Name: checkPrivateKey, Status: StatusFail, Message: message})
			return appendSkipped(checks, checkGitHubReachable, checkCredentialsValid, checkRunnerPermission, checkRunnerGroup)
		}
		checks = append(checks, Check{Name: checkPrivateKey, Status: StatusPass, Message: auth.GitHubApp.PrivateKeyPath})
	}

	group := target.RunnerGroup
	if target.Repo != "" || group == "" {
		group = "Default"
	}

	client, err := githubclient.New(target.configURL(), auth, scaleset.WithRetryableHTTPClint(noRetryClient()))
	if err == nil {
		_, err = client.GetRunnerGroupByName(ctx, group)
	}
	if err != nil {
		failed, message := classifyGitHubError(err.Error(), target, group)
		return appendOutcome(checks, failed, message)
	}

	checks = append(checks,
		Check{Name: checkGitHubReachable, Status: StatusPass, Message: "api.github.com answered"},
		Check{Name: checkCredentialsValid, Status: StatusPass, Message: "credentials accepted"},
		Check{Name: checkRunnerPermission, Status: StatusPass, Message: "runner admin permission confirmed for " + target.Name()},
	)
	if target.Repo != "" {
		checks = append(checks, Check{Name: checkRunnerGroup, Status: StatusPass, Message: "repositories use the Default group"})
	} else {
		checks = append(checks, Check{Name: checkRunnerGroup, Status: StatusPass, Message: "runner group " + group + " exists"})
	}
	return checks
}

// Passed returns true when every check passed.
func Passed(checks []Check) bool {
	for _, check := range checks {
		if check.Status == StatusFail {
			return false
		}
	}
	return true
}

// FirstFailure returns the message of the first failed check, or empty.
func FirstFailure(checks []Check) string {
	for _, check := range checks {
		if check.Status == StatusFail {
			return check.Name + ": " + check.Message
		}
	}
	return ""
}

func noRetryClient() *retryablehttp.Client {
	rc := retryablehttp.NewClient()
	rc.RetryMax = 0
	rc.HTTPClient.Timeout = githubTimeout
	return rc
}

func checkPrivateKeyFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return "private key " + path + " cannot be read: " + err.Error()
	}
	block, _ := pem.Decode(data)
	if block == nil || !strings.Contains(block.Type, "PRIVATE KEY") {
		return "private key " + path + " must contain a valid PEM private key"
	}
	return ""
}

// classifyGitHubError maps a scale set client error to the check that failed
// and a message a user can act on.
func classifyGitHubError(message string, target GitHubTarget, group string) (failedCheck, humanMessage string) {
	hasStatus := strings.Contains(message, "status=")
	switch {
	case !hasStatus && containsAny(message, "dial", "no such host", "timeout", "context deadline"):
		return checkGitHubReachable, "cannot reach api.github.com: " + message
	case strings.Contains(message, "failed to fetch access token") || strings.Contains(message, `status="401`):
		return checkCredentialsValid, "GitHub rejected the credentials (401). Check the token, or the App client ID, installation ID and key"
	case strings.Contains(message, `status="403`):
		return checkRunnerPermission, "credentials lack self hosted runner admin permission for " + target.Name() + " (403)"
	case strings.Contains(message, `status="404`):
		return checkRunnerPermission, target.Name() + " was not found or the credentials cannot manage its runners (404)"
	case strings.Contains(message, "no runner group found"):
		return checkRunnerGroup, "runner group " + group + " does not exist in org " + target.Org
	default:
		return checkRunnerPermission, message
	}
}

func containsAny(message string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(message, needle) {
			return true
		}
	}
	return false
}

// appendOutcome marks checks before the failed one as passed, the failed one
// as failed, and the rest as skipped.
func appendOutcome(checks []Check, failed, message string) []Check {
	order := []string{checkGitHubReachable, checkCredentialsValid, checkRunnerPermission, checkRunnerGroup}
	reached := false
	for _, name := range order {
		switch {
		case name == failed:
			checks = append(checks, Check{Name: name, Status: StatusFail, Message: message})
			reached = true
		case reached:
			checks = append(checks, Check{Name: name, Status: StatusSkipped})
		default:
			checks = append(checks, Check{Name: name, Status: StatusPass})
		}
	}
	return checks
}

func appendSkipped(checks []Check, names ...string) []Check {
	for _, name := range names {
		checks = append(checks, Check{Name: name, Status: StatusSkipped})
	}
	return checks
}
