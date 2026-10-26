# Project Notes

## Coherence

Read this first: it is the reason for every rule below.

- **What it is.** Decoherence is the accumulation of locally valid but mutually incompatible or redundant models and patterns in one system: four ways to handle errors, three ways to load config, two data models for the same thing. Each one works on its own; together they make every change expensive.
- **Why it compounds.** Every extra way of doing something has to be understood, and translated to and from, by every later change. Paths get longer, connections multiply, and each change switches between paradigms, so the cost of change grows faster than linearly with decoherence. It grows with change, not with time: the faster the code changes, the faster it decays.
- **Why AI agents must care most.** An agent has no memory across sessions. Each session re-infers the conventions from the small slice of code it read, and small differences between those inferences pile up into parallel patterns. Agents also change code fast, which makes them the largest source of decoherence. Left unchecked, the system slides into a state that only agents can change and that agents can no longer untangle (Decoherence Collapse).
- **So:** consistency beats local optimality. Fit new code into the existing patterns, even when another approach looks slightly better in isolation. When you touch an area with duplicate patterns, unify one small instance in the same PR and say so in the description (recohere). A PR that lowers coherence is a defect, even if it works.
- Source: [The Decoherence Trap](https://maevyn.codes/decoherence/).

The sections below (Standard patterns, Find the existing pattern first, Review checklist) apply this one.

## Build

- `make build` — Build dashboard + Go binary to `output/`
- `make test` — Run all tests
- `make check` — Quick local check (fmt + vet + build)

## Pre-commit

- Run `prek run --all-files` before commits to execute all pre-commit hooks (formatting, linting, etc.)

## Conventions

- Error messages must include context (relevant values) and be descriptive enough to diagnose without source code
## Standard patterns

| Concern | Use |
| --- | --- |
| Error handling | `fmt.Errorf("<action> <subject %s>: %w", ..., err)` with the relevant values; package-level `ErrX = errors.New(...)` for sentinels; see `internal/storage/storage.go`, `internal/auth/service.go` |
| API errors | Map to `connect.NewError(connect.Code..., err)` in the handler; see `internal/api/server.go` |
| Config loading | `config.Load` (viper) for reading, `config.ValidateYAML` / `config.ValidateConfig` for validation, `configstate` for writes and revisions; see `config/load.go`, `config/validation.go`, `internal/configstate/storage.go`. Current: `ValidateYAML`/`ValidateConfig` (see `config/validation.go`); Legacy: `(*Config).Validate` and its `validateOrg`/`validateRepo`/`validateAuth`/`validateRunnerSet` helpers (see `config/config.go`, called from `cmd/elastic-fruit-runner/daemon.go`); migrate when touched |
| Database / state | Open with `storage.Open` (SQLite, goose migrations); query only through sqlc (`sqlcdb.New(db)`), add SQL to `queries.sql` and run `make sqlc-generate`; see `internal/storage/storage.go`, `internal/storage/sqlc/queries.sql`, `internal/management/provisional.go` |
| Logging | `log/slog` default logger (JSON handler set in `cmd/`); never pass loggers as parameters; structs that need fixed context build `slog.Default().With(...)` in their constructor; see `internal/controller/controller.go`, `internal/management/jobs.go` |
| Retries & timeouts | Wrap request/response calls in `context.WithTimeout` (see `internal/cloudagent/commands.go`, `internal/probe/backend.go`). Exception: long-lived streams get no overall deadline, only an idle timeout that cancels the stream (see `watchCommands` / `streamIdleTimeout` in `internal/cloudagent/commands.go`). Retry with context-aware exponential backoff (+ jitter for fleet-wide calls). Current: see `internal/cloudagent/commands.go` (`nextReconnectDelay`, `withJitter`, `sleepWithContext`), `internal/tart/manager.go`; Legacy: fixed `time.Sleep` retry that ignores `ctx` (see `internal/management/service.go` `runController`); migrate when touched |
| HTTP clients | GitHub Actions API via `githubclient.New` (see `internal/githubclient/client.go`); cloud via the connect client built in `internal/cloudagent/client.go`. Current: construct a client per target (see `internal/cloudagent/client.go`); Legacy: `http.DefaultClient` with no timeout (see `internal/backend/runner_version.go`); migrate when touched |
| External commands | `exec.CommandContext(ctx, binpath.Lookup("<bin>"), ...)`; see `internal/backend/docker.go`, `internal/binpath/lookup.go` |
| CLI | cobra subcommands built by a `newXCommand()` constructor in their own file and registered in `newRootCommand`; flags bound through viper; see `cmd/elastic-fruit-runner/main.go`, `cmd/elastic-fruit-runner/enroll.go` |
| Console API | Define RPCs in `proto/controlplane/v1/controlplane.proto`, regenerate `gen/` with buf; dashboard calls them through `rpc()` in `dashboard/src/api/fetchers.ts` and `useSWR`; see `internal/api/server.go`, `dashboard/src/hooks/useDashboardSync.ts` |
| Testing | Standard `testing` (no assertion library), table-driven `t.Run`, real SQLite via `storage.Open` in `t.TempDir()`, `httptest` for HTTP; see `config/config_test.go`, `internal/management/service_test.go`, `internal/api/server_test.go`. Integration tests are godog features in `features/` with steps in `test/integration/steps_test.go` |

## Find the existing pattern first

- Before implementing anything, search the repo for how the same or a similar thing is already done (`rg`, the table above) and follow that pattern.
- Introducing a new pattern for an existing concern requires a justification in the PR description. The same PR must migrate every caller of the old pattern and delete it, so only one way remains.
- Every PR description must include a line `Reused pattern: <pattern and file it follows>` (or the justification for a new one).

## Review checklist

Reviewers must check whether the PR introduces any of the following. If it does and the PR does not justify it and migrate all callers, request changes.

- A new concept (package, abstraction, config key, or service) where an existing one already covers the need.
- A new way of doing an existing thing (error handling, config, storage, logging, retries, HTTP, tests) that differs from "Standard patterns".
- A duplicate model or type (Go struct, proto message, or dashboard type in `dashboard/src/types.ts`) that mirrors an existing one.
- A missing `Reused pattern:` line in the PR description.
