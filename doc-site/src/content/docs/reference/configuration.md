---
title: Configuration Reference
description: YAML fields, defaults, limits, and validation rules for Elastic Fruit Runner.
---

Elastic Fruit Runner reads one YAML config file. Unknown fields, duplicate keys, and multiple YAML documents are errors.

## Config file search

Without `--config`, the daemon checks these paths in order:

1. `~/.elastic-fruit-runner/config.yaml`
2. `/opt/homebrew/var/elastic-fruit-runner/config.yaml`
3. `/usr/local/var/elastic-fruit-runner/config.yaml`
4. `/etc/elastic-fruit-runner/config.yaml`

Use `--config PATH` to select another file.

## Complete example

```yaml
orgs:
  - org: your-org
    auth:
      pat_token: ghp_replace_me
    runner_group: Default
    runner_sets:
      - name: efr-macos-arm64
        backend: tart
        image: ghcr.io/cirruslabs/macos-tahoe-xcode:26.3
        labels: [self-hosted, macos, arm64]
        max_runners: 2

repos:
  - repo: your-org/your-repo
    auth:
      pat_token: ghp_replace_me
    runner_sets:
      - name: efr-repo-linux-arm64
        backend: docker
        image: ghcr.io/actions-runner-controller/actions-runner-controller/actions-runner-dind:latest
        labels: [self-hosted, linux, arm64]
        max_runners: 4
        platform: linux/arm64

idle_timeout: 15m
log_level: info
api_addr: 127.0.0.1:8080
db_path: ./jobs.db
log_path: ./elastic-fruit-runner.log

cors:
  allow_origin: "https://runner-console.example.com"
  allow_methods: "GET, POST, OPTIONS"
  allow_headers: "Content-Type, Connect-Protocol-Version, X-CSRF-Token"
  expose_headers: "Connect-Protocol-Version"
  allow_credentials: true
  max_age: 3600
```

At least one item must exist in `orgs` or `repos`.

## Top level fields

| Field | Type | Default | Rules |
|---|---|---|---|
| `orgs` | list | empty | Organization runner scopes |
| `repos` | list | empty | Repository runner scopes |
| `idle_timeout` | duration | `15m` | Greater than zero and no more than `24h` |
| `log_level` | string | `info` | `debug`, `info`, `warn`, or `error` |
| `api_addr` | string | `:8080` | Host and port accepted by the Go network listener |
| `db_path` | string | `~/.elastic-fruit-runner/jobs.db` | Writable file path or `:memory:` |
| `log_path` | string | empty | Optional writable log file. Empty sends logs to standard output |
| `cors` | object | runtime defaults | CORS response settings |

## Organization fields

Each `orgs[]` item supports:

| Field | Type | Required | Rules |
|---|---|---|---|
| `org` | string | yes | Valid GitHub organization name |
| `auth` | object | yes | Exactly one auth method |
| `runner_group` | string | no | Defaults to `Default` |
| `runner_sets` | list | yes | At least one runner set |

An organization name uses GitHub owner rules. It contains 1 to 39 letters, numbers, or single hyphen characters. It cannot start or end with a hyphen.

## Repository fields

Each `repos[]` item supports:

| Field | Type | Required | Rules |
|---|---|---|---|
| `repo` | string | yes | `owner/repository` |
| `auth` | object | yes | Exactly one auth method |
| `runner_sets` | list | yes | At least one runner set |

The repository part contains 1 to 100 letters, numbers, dots, underscores, or hyphen characters.

Repository runner sets use the default runner group.

## Auth fields

Configure exactly one of `pat_token` or `github_app`. They are mutually exclusive.

### Personal Access Token

| Field | Type | Required | Rules |
|---|---|---|---|
| `pat_token` | string | yes | Nonempty |

For an organization scope, the token needs Organization Self hosted runners read and write access.

For a repository scope, the token also needs repository Administration read and write access.

### GitHub App

| Field | Type | Required | Rules |
|---|---|---|---|
| `github_app.client_id` | string | yes | Nonempty |
| `github_app.installation_id` | integer | yes | Greater than zero |
| `github_app.private_key_path` | string | yes | Readable PEM private key file |

The private key PEM block type must contain `PRIVATE KEY`.

See [How to configure GitHub App authentication](/how-to/configure-github-app/) for the setup steps and permissions.

## Runner set fields

Each `runner_sets[]` item supports:

| Field | Type | Required | Rules |
|---|---|---|---|
| `name` | string | yes | Nonempty and unique across the whole config |
| `backend` | string | yes | `docker` or `tart` |
| `image` | string | yes | Nonempty image reference |
| `labels` | list of strings | no | GitHub runner labels |
| `max_runners` | integer | yes | From 1 through 1000 |
| `platform` | string | no | Backend specific |

### Docker

`platform` can be empty. When set, it must start with `linux/`, such as `linux/arm64` or `linux/amd64`.

The image must contain the GitHub Actions runner and the tools needed by the workflow.

### Tart

`platform` must be empty. The image is a local or OCI Tart VM image.

Tart requires Apple Silicon and the Tart CLI on the host.

## CORS fields

The server applies these runtime defaults when a field is empty:

| Field | Type | Runtime default | Rules |
|---|---|---|---|
| `allow_origin` | string | `*` | `*` or one valid origin URL |
| `allow_methods` | string | `GET, POST, OPTIONS` | Only `GET`, `POST`, and `OPTIONS` |
| `allow_headers` | string | `Content-Type, Connect-Protocol-Version, X-CSRF-Token` | One line |
| `expose_headers` | string | `Connect-Protocol-Version` | One line |
| `allow_credentials` | boolean | `false` | Requires a specific `allow_origin` when true |
| `max_age` | integer | `0` | From 0 through 86400 seconds |

Keep the Console on the same origin when possible. The daemon does not provide TLS.

## Path validation

For `db_path`, `log_path`, and GitHub App private key paths, validation checks the local file system.

* An existing file must be usable for its purpose.
* A configured file path cannot name a directory.
* An existing direct parent directory must be writable.
* If the direct parent does not exist, its parent must exist.
* `:memory:` is allowed for `db_path`.

## Strict validation

Startup, Console validation, and Console save use the same strict validator.

Validation checks:

* YAML structure
* Known and duplicate fields
* Required fields
* Duration and number limits
* GitHub owner and repository names
* Global runner set name uniqueness
* Auth method choice
* Private key file and PEM data
* Backend, image, and platform rules
* API address
* CORS values
* Writable storage paths

GitHub connectivity is not checked. Validation returns a warning for this limit.

See [How to edit and activate config](/how-to/edit-config/) for the save flow.
