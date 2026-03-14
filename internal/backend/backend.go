package backend

import "context"

// Backend abstracts the runner execution environment.
// Implementations handle the full lifecycle: prepare environment, run the
// GitHub Actions runner with a JIT config, and clean up afterwards.
type Backend interface {
	// Prepare sets up the execution environment for a runner instance.
	// For Tart this means clone + start VM; for host mode this is a no-op.
	Prepare(ctx context.Context, name string) error

	// RunRunner downloads (if needed) and starts the GitHub Actions runner
	// binary with the given JIT config. Blocks until the runner exits.
	RunRunner(ctx context.Context, name, jitConfig string) error

	// Cleanup tears down the execution environment.
	// Must be safe to call even if Prepare or RunRunner failed.
	Cleanup(ctx context.Context, name string)
}
