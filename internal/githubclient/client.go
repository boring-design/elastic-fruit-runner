package githubclient

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/actions/scaleset"

	"github.com/boring-design/elastic-fruit-runner/config"
)

// New builds a GitHub Actions scale set client for one org or repo config URL.
func New(configURL string, auth *config.AuthConfig, options ...scaleset.HTTPOption) (*scaleset.Client, error) {
	switch auth.Mode() {
	case config.AuthModeGitHubApp:
		pemBytes, readErr := os.ReadFile(auth.GitHubApp.PrivateKeyPath)
		if readErr != nil {
			return nil, fmt.Errorf("read GitHub App private key %s: %w", auth.GitHubApp.PrivateKeyPath, readErr)
		}
		slog.Info("authenticating with GitHub App",
			"configURL", configURL,
			"clientID", auth.GitHubApp.ClientID,
			"installationID", auth.GitHubApp.InstallationID,
		)
		return scaleset.NewClientWithGitHubApp(scaleset.ClientWithGitHubAppConfig{
			GitHubConfigURL: configURL,
			GitHubAppAuth: scaleset.GitHubAppAuth{
				ClientID:       auth.GitHubApp.ClientID,
				InstallationID: auth.GitHubApp.InstallationID,
				PrivateKey:     string(pemBytes),
			},
		}, options...)
	case config.AuthModePAT:
		slog.Info("authenticating with PAT", "configURL", configURL)
		return scaleset.NewClientWithPersonalAccessToken(
			scaleset.NewClientWithPersonalAccessTokenConfig{
				GitHubConfigURL:     configURL,
				PersonalAccessToken: *auth.PATToken,
			},
			options...,
		)
	default:
		return nil, fmt.Errorf("unknown auth mode %q", auth.Mode())
	}
}
