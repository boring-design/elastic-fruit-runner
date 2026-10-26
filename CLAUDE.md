# Project Notes

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
