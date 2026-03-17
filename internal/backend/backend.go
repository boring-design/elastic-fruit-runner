package backend

import "context"

// Backend abstracts the runner execution environment.
// Implementations handle the full lifecycle: prepare environment, start the
// GitHub Actions runner with a JIT config, and clean up afterwards.
type Backend interface {
	// Prepare sets up the execution environment for a runner instance.
	// For Tart this means clone + start VM + wait for IP.
	Prepare(ctx context.Context, name string) error

	// StartRunner launches the GitHub Actions runner inside the environment
	// with the given JIT config. Returns immediately after the runner
	// process is started; the runner executes asynchronously inside the VM.
	StartRunner(ctx context.Context, name, jitConfig string) error

	// Cleanup tears down the execution environment.
	// Must be safe to call even if Prepare or StartRunner failed.
	Cleanup(ctx context.Context, name string)
}
