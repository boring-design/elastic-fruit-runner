---
title: Getting Started
description: Install Elastic Fruit Runner on an Apple Silicon Mac, open the Console, and run your first job.
---

This tutorial takes you from an empty Mac to one completed GitHub Actions job. You will also use the Console to see what the runner is doing.

## What you need

* A Mac with Apple Silicon
* Homebrew
* Tart
* A GitHub organization where you have admin access
* A GitHub Personal Access Token with Organization Self hosted runners read and write access

Install Tart before you continue:

```sh
brew install cirruslabs/cli/tart
```

Check that Tart is ready:

```sh
tart --version
```

You should see a version number.

## Install Elastic Fruit Runner

```sh
brew install boring-design/tap/elastic-fruit-runner
```

Check the installed command:

```sh
elastic-fruit-runner --help
```

The command should print its available options.

## Create the config

Create the config directory:

```sh
mkdir -p ~/.elastic-fruit-runner
```

Create `~/.elastic-fruit-runner/config.yaml` with this content. Replace `your-org` and `ghp_xxx` with your own values.

```yaml
orgs:
  - org: your-org
    auth:
      pat_token: ghp_xxx
    runner_group: Default
    runner_sets:
      - name: efr-macos-arm64
        backend: tart
        image: ghcr.io/cirruslabs/macos-tahoe-xcode:26.3
        labels: [self-hosted, macos, arm64]
        max_runners: 2

idle_timeout: 15m
log_level: info
```

Elastic Fruit Runner checks the config when it starts. If a field is not valid, the local log names the field and the problem.

## Start the service

```sh
brew services start elastic-fruit-runner
```

Check the service:

```sh
brew services info elastic-fruit-runner
```

The output should show `Running: true`.

## Set up the Console

The first start writes a one time setup code to the local log.

```sh
grep '"msg":"console admin setup required"' /opt/homebrew/var/log/elastic-fruit-runner.log | tail -n 1
```

Open [http://127.0.0.1:8080](http://127.0.0.1:8080). Enter the setup code, choose an admin password, and sign in.

The Overview page should show:

* One configured runner set
* The GitHub connection state
* No running job yet
* Current host resources
* Config state `In sync`

## Run your first workflow

In a repository that belongs to the configured organization, create `.github/workflows/test-efr.yaml`:

```yaml
name: Test Elastic Fruit Runner
on:
  workflow_dispatch:

jobs:
  hello:
    runs-on: efr-macos-arm64
    steps:
      - run: |
          echo "Hello from Elastic Fruit Runner"
          sw_vers
          uname -m
```

Open the Actions page in GitHub, select **Test Elastic Fruit Runner**, and choose **Run workflow**.

## Watch the job

Return to the Console and open **Jobs**. The new job moves through its lifecycle while the Tart VM starts and runs the workflow.

Open the job to see:

* Repository and workflow name
* Runner set and runner name
* Start and completion times
* Runner log
* Available resource data
* A link back to GitHub Actions

## Confirm the result

Wait for the job to finish. The Jobs page should show `Success`.

Open the job in GitHub Actions. Its output should contain the macOS version and `arm64`.

You now have an elastic macOS runner and a working operations Console.

## Continue

* [Use the operations Console](/how-to/use-console/)
* [Configure GitHub App authentication](/how-to/configure-github-app/)
* [Manage multiple organizations and repositories](/how-to/multiple-orgs-repos/)
* [Read the configuration reference](/reference/configuration/)
