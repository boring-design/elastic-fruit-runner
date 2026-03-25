---
title: Getting Started
description: Set up elastic-fruit-runner on macOS and run your first GitHub Actions job on an ephemeral Tart VM.
---

This tutorial walks you through installing elastic-fruit-runner on a macOS Apple Silicon machine, configuring it, and running your first GitHub Actions workflow on an ephemeral Tart VM.

By the end, you will have a working self-hosted runner that automatically scales up VMs for incoming jobs and destroys them after completion.

## Prerequisites

- A Mac with Apple Silicon (M1 or later)
- [Homebrew](https://brew.sh) installed
- [Tart](https://tart.run) installed (`brew install cirruslabs/cli/tart`)
- A GitHub organization or repository where you have admin access
- A GitHub Personal Access Token (PAT) with **Organization > Self-hosted runners: Read and write** scope

## Step 1: Install elastic-fruit-runner

```sh
brew install boring-design/tap/elastic-fruit-runner
```

Verify the installation:

```sh
elastic-fruit-runner --help
```

## Step 2: Create a configuration file

```sh
mkdir -p ~/.elastic-fruit-runner
```

Create `~/.elastic-fruit-runner/config.yaml` with the following content. Replace `your-org` with your GitHub organization name and `ghp_xxx` with your PAT:

```yaml
orgs:
  - org: your-org
    auth:
      pat_token: ghp_xxx
    runner_group: Default
    runner_sets:
      # macOS runners via Tart VMs (Apple Silicon only)
      - name: efr-macos-arm64
        backend: tart
        image: ghcr.io/cirruslabs/macos-tahoe-xcode:26.3
        labels: [self-hosted, macos, arm64]
        max_runners: 2

      # Linux arm64 runners via Docker
      - name: efr-linux-arm64
        backend: docker
        image: ghcr.io/actions-runner-controller/actions-runner-controller/actions-runner-dind:latest
        labels: [self-hosted, linux, arm64]
        max_runners: 4
        platform: linux/arm64

      # Linux amd64 runners via Docker (Rosetta 2 emulation on Apple Silicon)
      - name: efr-linux-amd64
        backend: docker
        image: ghcr.io/actions-runner-controller/actions-runner-controller/actions-runner-dind:latest
        labels: [self-hosted, linux, amd64]
        max_runners: 4
        platform: linux/amd64

idle_timeout: 15m
log_level: info
```

## Step 3: Start the service

Start elastic-fruit-runner as a background service that auto-starts on login:

```sh
brew services start elastic-fruit-runner
```

Check that it is running:

```sh
brew services info elastic-fruit-runner
```

You should see `elastic-fruit-runner` listed as `started`.

## Step 4: Verify in the logs

Watch the logs to confirm the daemon connected to GitHub:

```sh
tail -f /opt/homebrew/var/log/elastic-fruit-runner.log
```

You should see log lines indicating the scale set was registered and the daemon is polling for jobs.

## Step 5: Run a workflow

In your GitHub repository, create a workflow file `.github/workflows/test-efr.yaml`:

```yaml
name: Test elastic-fruit-runner
on: workflow_dispatch

jobs:
  hello:
    runs-on: efr-macos-arm64
    steps:
      - run: |
          echo "Hello from elastic-fruit-runner!"
          sw_vers
          uname -m
```

Go to your repository on GitHub, navigate to **Actions**, select the **Test elastic-fruit-runner** workflow, and click **Run workflow**.

Switch back to the log output. You should see:
1. The daemon detecting the new job
2. A Tart VM being cloned and started
3. The runner registering and executing the job
4. The VM being destroyed after completion

## Step 6: Check the workflow result

Go back to GitHub Actions. The workflow run should show a green checkmark, with output displaying the macOS version and `arm64` architecture.

## Next steps

- [Configure GitHub App auth](/how-to/configure-github-app/) for production org deployments
- [Deploy on Linux with Docker](/how-to/install-linux-docker/) for Linux arm64/amd64 runners
- [What is elastic-fruit-runner?](/explanation/what-is-efr/) to understand the project
