# elastic-fruit-runner — Architecture

## Overview

`elastic-fruit-runner` is a daemon that acts as a GitHub Actions **Runner Scale Set** controller for Apple Silicon Macs. It uses Tart to create and destroy ephemeral macOS VMs, each running a one-shot actions/runner configured with a **JIT (Just-In-Time) token** so it accepts exactly one job before self-terminating.

```
GitHub Actions ←→  Scale Set API  ←→  Daemon  ←→  Tart VMs
                   (long-polling)              (ephemeral, per-job)
```

---

## Component Design

### `cmd/elastic-fruit-runner/main.go`
Entry point. Parses config (flags + env vars), builds the authenticated `scaleset.Client`, wires everything together, and handles SIGINT/SIGTERM via `signal.NotifyContext`.

### `config/config.go`
Flat `Config` struct populated from `flag` + `os.Getenv`. Flags take precedence over env vars. Key knobs:

| Flag | Env | Default | Notes |
|------|-----|---------|-------|
| `--token` | `GITHUB_TOKEN` | — | PAT with `manage_runners:org` scope |
| `--url` | `GITHUB_CONFIG_URL` | — | e.g. `https://github.com/myorg` |
| `--runner-group` | `GITHUB_RUNNER_GROUP` | `Default` | |
| `--scale-set-name` | `SCALE_SET_NAME` | `elastic-fruit-runner` | |
| `--vm-image` | `TART_VM_IMAGE` | cirruslabs sequoia base | Base image to clone |
| `--max-runners` | — | `2` | Apple EULA caps macOS VMs at 2 per host |

### `internal/daemon/daemon.go`
The core control loop. Implements `listener.Scaler`:

1. **Bootstrap** — resolves the runner group, then gets or creates the named scale set via the scaleset API.
2. **Message session** — opens a `MessageSessionClient` (manages the long-poll session token lifecycle automatically).
3. **Listener** — the `listener.Listener` from `github.com/actions/scaleset/listener` handles polling and message acknowledgement. It calls back into `Daemon` via the `Scaler` interface.
4. **`HandleDesiredRunnerCount`** — spawns N goroutines (capped at `MaxRunners`), each running `spawnRunner`. This is the scaling trigger point.
5. **`HandleJobStarted` / `HandleJobCompleted`** — log-only in the PoC; used for observability hooks in production.

### `internal/tart/manager.go`
Thin wrapper around the `tart` CLI (assumed installed on host). All operations use `exec.CommandContext` so they respect context cancellation. Operations:

- `Clone(image, name)` — creates an ephemeral copy of a base image
- `Start(name)` — runs the VM headlessly in a background goroutine
- `IPAddress(name)` — waits up to 60 s for DHCP
- `Exec(name, cmd...)` — runs a command inside the VM via `tart exec`
- `Stop(name)` + `Delete(name)` — cleanup (always deferred in `spawnRunner`)

### `internal/runner/register.go`
Downloads `actions/runner` (ARM64 macOS tarball) into the VM if not already present (enables pre-warmed image optimisation later), then calls `config.sh --jitconfig` and `run.sh`. The JIT config makes the runner ephemeral — it registers, picks up one job, runs it, and exits. The `StartWithJITConfig` call blocks until the runner exits, which is when `spawnRunner` proceeds to VM cleanup.

---

## Data Flow (per job)

```
1. GitHub enqueues a job targeting "elastic-fruit-runner" scale set
2. listener.Listener detects it via long-poll → calls HandleDesiredRunnerCount(n)
3. Daemon spawns goroutine → spawnRunner()
4. tart clone <base-image> efr-<ts>-<id>
5. tart run efr-... --no-graphics       (background)
6. tart ip efr-... --wait 60            (wait for network)
7. scaleset.Client.GenerateJitRunnerConfig() → EncodedJITConfig
8. tart exec efr-... -- bash -c "curl runner; config.sh --jitconfig ...; run.sh"
9. Runner registers with GitHub, job executes inside VM
10. run.sh exits → tart stop + tart delete
```

---

## Key Design Decisions

### Why `github.com/actions/scaleset`?
GitHub published this official Go client in February 2026 specifically for custom autoscalers. It handles session management, token refresh, long-poll retries, and message acknowledgement — removing significant protocol complexity from this daemon.

### Why JIT config instead of pre-registration?
JIT tokens create ephemeral, single-use runners that self-deregister after one job. This means no stale runner entries in GitHub's runner list, no manual cleanup of runner registrations, and no risk of a VM crash leaving a "ghost" runner that blocks queue draining.

### Why `tart exec` for runner setup?
`tart exec` runs commands directly in the VM without needing SSH key management or knowing the VM's IP for SSH. It's the idiomatic Tart approach and keeps the host side stateless.

### Concurrency model
Each job gets its own goroutine → Tart VM → runner process. There is no shared mutable state between goroutines other than the atomic `vmCounter`. The `MaxRunners` cap (default 2) aligns with the Apple EULA limit for macOS virtualisation on a single host.

### Graceful shutdown
`signal.NotifyContext` cancels the root context on SIGINT/SIGTERM. The listener stops polling, and context cancellation propagates into any in-flight `tart exec` commands. Deferred `Stop`/`Delete` calls in `spawnRunner` use a fresh `context.Background()` to ensure cleanup even after the root context is cancelled.

---

## What's Not Built Yet

- **Warm pool / pre-cloning** — cloning takes ~10–30 s; pre-warming base VMs would reduce job start latency.
- **Linux runners** — Docker/OrbStack runners for arm64 and Rosetta-emulated amd64.
- **GitHub App auth** — currently PAT only; App auth is preferred for org-wide deployments.
- **Production retries** — the scaleset client has configurable retry logic; we use defaults.
- **Observability** — `HandleJobStarted`/`Completed` are log-only; could emit metrics to Prometheus.
- **GUI** — Wails-based status dashboard is a future milestone.
