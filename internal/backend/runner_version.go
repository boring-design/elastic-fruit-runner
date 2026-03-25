package backend

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
)

var (
	resolvedVersion string
	versionOnce     sync.Once
)

// ResolveRunnerVersion fetches the latest actions/runner release tag from
// GitHub and caches the result for the lifetime of the process.
func ResolveRunnerVersion(ctx context.Context) (string, error) {
	var resolveErr error
	versionOnce.Do(func() {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet,
			"https://api.github.com/repos/actions/runner/releases/latest", http.NoBody)
		if err != nil {
			resolveErr = err
			return
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			resolveErr = fmt.Errorf("fetch latest runner release: %w", err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			resolveErr = fmt.Errorf("GitHub API returned %s", resp.Status)
			return
		}

		var release struct {
			TagName string `json:"tag_name"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
			resolveErr = fmt.Errorf("decode release response: %w", err)
			return
		}

		resolvedVersion = strings.TrimPrefix(release.TagName, "v")
	})

	if resolveErr != nil {
		return "", resolveErr
	}
	return resolvedVersion, nil
}
