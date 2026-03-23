package backend

import "context"

// Backend abstracts the runner execution environment.
// Implementations handle the full lifecycle: start the GitHub Actions runner
// with a JIT config, and clean up afterwards.
type Backend interface {
	// Run starts a runner instance with the given JIT config.
	// It sets up the execution environment and launches the GitHub Actions
	// runner process. Returns once the runner is up and accepting jobs.
	Run(ctx context.Context, name, jitConfig string) error

	// Cleanup tears down the execution environment.
	// Must be safe to call even if Run failed.
	Cleanup(ctx context.Context, name string)

	// CleanupAll removes all resources whose name starts with prefix.
	// Called once at controller startup to ensure a clean slate.
	CleanupAll(ctx context.Context, prefix string)
}
