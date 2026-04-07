---
title: How to run integration tests locally
description: Set up and run the full integration test suite against real GitHub Actions and Docker.
---

Integration tests verify the complete elastic-fruit-runner lifecycle: registering a runner scale set, receiving workflow jobs from GitHub Actions, launching Docker runners, and cleaning up. They run against real infrastructure — no mocks.

## Prerequisites

- **Docker** running locally
- **GitHub classic PAT** with scopes: `admin:org`, `repo`, `workflow`, for the `boring-design` org
- **GitHub App** private key (`.pem`), Client ID, and Installation ID for the [`efr-integration-test`](https://github.com/organizations/boring-design/settings/apps/efr-integration-test) app
- Access to [`boring-design/efr-integration-test`](https://github.com/boring-design/efr-integration-test) repo (contains the target workflow)

All credentials are stored in 1Password under **"efr-integration-test-ci PAT"** in the Private vault.

## Setup

Create `.env.integration-test` in the project root (gitignored):

```bash
# Required
EFR_TEST_CONFIG_URL=https://github.com/boring-design
EFR_TEST_WORKFLOW_ORG=boring-design
EFR_TEST_WORKFLOW_REPO=efr-integration-test
EFR_TEST_WORKFLOW_TOKEN=<your-classic-pat>

# PAT auth
EFR_TEST_PAT=<your-classic-pat>

# GitHub App auth
EFR_TEST_APP_CLIENT_ID=<app-client-id>
EFR_TEST_APP_INSTALLATION_ID=<installation-id>
EFR_TEST_APP_PRIVATE_KEY_PATH=/absolute/path/to/github-app-key.pem
```

Place the GitHub App private key at the path above. The default location `github-app-key.pem` in the project root is gitignored.

:::caution
`EFR_TEST_APP_PRIVATE_KEY_PATH` must be an **absolute path**. The test working directory is `test/integration/`, so relative paths won't resolve correctly.
:::

## Running

```bash
make integration-test
```

This sources `.env.integration-test` and runs:

```bash
go test -tags=integration -v -count=1 -timeout=15m ./test/integration/
```

If any required variable is missing, the test **fails immediately** — it does not skip.

A typical run takes about **70 seconds** (two end-to-end controller scenarios running sequentially).

## How it works

Tests are written in [Gherkin](https://cucumber.io/docs/gherkin/) (`.feature` files under `features/`) and executed by godog. The test files use the `//go:build integration` build tag, so `go test ./...` never touches them.

### Test scenarios

| Feature | Scenarios | What it covers |
|---|---|---|
| `config/loading` | 5 | YAML parsing, defaults, env overrides |
| `binpath/lookup` | 4 | Binary path resolution and caching |
| `jobstore/lifecycle` | 7 | SQLite job store CRUD, ordering, concurrency |
| `vitals/collector` | 1 | System metrics collection |
| `controller/lifecycle` | 2 | Full controller lifecycle with PAT and GitHub App auth |

### Controller lifecycle flow

The two controller scenarios (PAT auth + GitHub App auth) each execute the same flow:

1. Register a runner scale set with a random name (e.g. `efr-test-a1b2c`)
2. Start the controller and connect to the GitHub Actions broker
3. Dispatch the `test-job.yaml` workflow in `boring-design/efr-integration-test`
4. The workflow runs on `[self-hosted, <scale-set-name>]`, ensuring it lands on our test runner
5. Wait for the workflow to complete successfully
6. Verify at least one job was recorded in the job store
7. Shut down the controller and delete the scale set

## Environment variables

| Variable | Required | Default | Description |
|---|---|---|---|
| `EFR_TEST_CONFIG_URL` | Yes | — | GitHub org URL |
| `EFR_TEST_WORKFLOW_ORG` | Yes | — | Org that owns the test workflow repo |
| `EFR_TEST_WORKFLOW_REPO` | Yes | — | Repo with the test workflow |
| `EFR_TEST_WORKFLOW_TOKEN` | Yes | — | PAT to trigger workflow dispatch |
| `EFR_TEST_PAT` | Yes | — | PAT for scale set client auth |
| `EFR_TEST_APP_CLIENT_ID` | Yes | — | GitHub App Client ID |
| `EFR_TEST_APP_INSTALLATION_ID` | Yes | — | GitHub App Installation ID |
| `EFR_TEST_APP_PRIVATE_KEY_PATH` | Yes | — | Absolute path to App private key |
| `EFR_TEST_WORKFLOW_FILE` | No | `test-job.yaml` | Workflow filename to dispatch |
| `EFR_TEST_RUNNER_IMAGE` | No | `ghcr.io/quipper/actions-runner:2.332.0` | Runner Docker image |
| `EFR_TEST_RUNNER_GROUP` | No | `Default` | Runner group name |

## CI

The same tests run in CI via `.github/workflows/controller-integration-test.yaml`. Credentials come from GitHub Secrets. The workflow triggers on pushes to `main` that touch relevant paths, or manually via `workflow_dispatch`.
