---
title: Configuration Reference
description: Complete reference for all elastic-fruit-runner configuration options.
---

All configuration is done through a YAML config file. The only CLI flag is `--config` to specify the file path.

## Config file search paths

If `--config` is not specified, the following paths are searched in order:

1. `~/.elastic-fruit-runner/config.yaml`
2. `/opt/homebrew/var/elastic-fruit-runner/config.yaml`
3. `/usr/local/var/elastic-fruit-runner/config.yaml`
4. `/etc/elastic-fruit-runner/config.yaml`

## Full example

```yaml
orgs:
  - org: your-org
    auth:
      # Option A: GitHub App (recommended)
      github_app:
        client_id: Iv1.xxxxxxxxxxxxxxxx
        installation_id: 12345678
        private_key_path: /path/to/private-key.pem
      # Option B: Personal Access Token (uncomment and remove github_app above)
      # pat_token: ghp_xxx
    runner_group: Default
    runner_sets:
      - name: efr-macos-arm64
        backend: tart
        image: ghcr.io/cirruslabs/macos-tahoe-xcode:26.3
        labels: [self-hosted, macos, arm64]
        max_runners: 2
      - name: efr-linux-arm64
        backend: docker
        image: ghcr.io/actions-runner-controller/actions-runner-controller/actions-runner-dind:latest
        labels: [self-hosted, linux, arm64]
        max_runners: 4
        platform: linux/arm64
      - name: efr-linux-amd64
        backend: docker
        image: ghcr.io/actions-runner-controller/actions-runner-controller/actions-runner-dind:latest
        labels: [self-hosted, linux, amd64]
        max_runners: 4
        platform: linux/amd64

repos:
  - repo: your-org/your-repo
    auth:
      pat_token: ghp_xxx
    runner_sets:
      - name: repo-runner
        backend: docker
        image: ghcr.io/actions-runner-controller/actions-runner-controller/actions-runner-dind:latest
        labels: [self-hosted, linux, arm64]
        max_runners: 2

idle_timeout: 15m
log_level: info
```

## Top-level fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `orgs` | list | — | Organization-level runner set configurations |
| `repos` | list | — | Repository-level runner set configurations |
| `idle_timeout` | duration | `15m` | Time after which idle runners are reaped. Must be > 0 |
| `log_level` | string | `info` | Log level: `debug`, `info`, `warn`, `error` |

At least one of `orgs` or `repos` must be configured.

## Organization configuration (`orgs[]`)

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `org` | string | yes | — | GitHub organization name |
| `auth` | object | yes | — | Authentication configuration |
| `runner_group` | string | no | `Default` | Runner group name |
| `runner_sets` | list | yes | — | At least one runner set |

## Repository configuration (`repos[]`)

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `repo` | string | yes | Repository in `owner/repo` format |
| `auth` | object | yes | Authentication configuration |
| `runner_sets` | list | yes | At least one runner set |

Repository-level runners always use the default runner group.

## Authentication (`auth`)

Exactly one of `pat_token` or `github_app` must be configured per org/repo. They are mutually exclusive.

### GitHub App

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `github_app.client_id` | string | yes | GitHub App Client ID (starts with `Iv1.`) |
| `github_app.installation_id` | integer | yes | Installation ID |
| `github_app.private_key_path` | string | yes | Path to the private key `.pem` file |

### Personal Access Token

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `pat_token` | string | yes | GitHub PAT (must not be empty) |

**Required PAT scopes:**
- Organization-level: **Organization > Self-hosted runners: Read and write**
- Repository-level: **Repository > Administration: Read and write**

## Runner set configuration (`runner_sets[]`)

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `name` | string | yes | — | Runner set name (used as `runs-on` label in workflows). Must be unique across all orgs/repos |
| `backend` | string | yes | — | Backend type: `tart`, `docker`, or `host` |
| `image` | string | no | — | VM image (Tart) or container image (Docker) |
| `labels` | list | no | — | Runner labels |
| `max_runners` | integer | yes | — | Maximum concurrent runners. Must be > 0 |
| `platform` | string | no | — | Docker platform (e.g., `linux/arm64`, `linux/amd64`) |

### Backend: `tart`

For macOS VMs on Apple Silicon. Uses [Tart](https://tart.run) to create ephemeral VMs.

- `image`: OCI image reference (e.g., `ghcr.io/cirruslabs/macos-tahoe-xcode:26.3`)
- `max_runners`: Recommend capping at 2 per host (Apple EULA restriction for macOS VMs)
- Requires the `tart` CLI installed on the host

### Backend: `docker`

For Linux containers. Uses Docker-in-Docker.

- `image`: Docker image with actions/runner (e.g., `ghcr.io/actions-runner-controller/actions-runner-controller/actions-runner-dind:latest`)
- `platform`: Specify for cross-architecture (e.g., `linux/amd64` for Rosetta 2 emulation on Apple Silicon)
- Requires Docker installed and running on the host

### Backend: `host`

Runs the runner directly on the host machine. Zero overhead but ties the runner to the host environment.

## Environment variables

Only one environment variable is supported:

| Variable | Config file equivalent | Description |
|----------|----------------------|-------------|
| `LOG_LEVEL` | `log_level` | Overrides the log level from config file |

All other configuration must be done through the YAML config file.
