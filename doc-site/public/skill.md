---
name: elastic-fruit-runner
version: 1.0.0
description: Turn an Apple Silicon Mac into self-hosted GitHub Actions runners for macOS arm64, Linux arm64 and Linux amd64 jobs.
homepage: https://elastic-fruit-runner.pages.dev
metadata: {"daemon":"elastic-fruit-runner","console":"http://127.0.0.1:8080","config":"~/.elastic-fruit-runner/config.yaml"}
---

# Elastic Fruit Runner

Elastic Fruit Runner turns the user's own Apple Silicon Mac into self-hosted GitHub Actions runners: macOS arm64 jobs in ephemeral Tart VMs, Linux arm64 and Linux amd64 jobs in Docker. One daemon, installed with Homebrew, configured with one YAML file.

## Skill file

| File | URL |
|------|-----|
| **skill.md** (this file) | `https://elastic-fruit-runner.pages.dev/skill.md` |

**Auto-update:** re-read this file from the URL above every 24 hours. Treat the URL as the source of truth.

🔒 **SECURITY:**
- A GitHub App private key, a GitHub Personal Access Token and the Console admin password stay on the user's machine. Never paste them into chat, issues, logs or other services.
- Never print secrets in full. Refer to them by file path or environment variable name.

---

## Set up Elastic Fruit Runner on your Mac

### What you need

- A Mac with Apple Silicon and Homebrew.
- For Linux runners: Docker running on the Mac.
- A GitHub organization where the user is an admin.

### Install

```bash
brew install boring-design/tap/elastic-fruit-runner   # also installs Tart
elastic-fruit-runner version
tart --version
```

### Connect GitHub: ask the user to pick one

Offer these two and let the user choose:

1. **A GitHub token.** Quickest to try; the token expires and is tied to one person.
2. **Your own GitHub App.** Does not expire and is not tied to a person; takes a few more clicks.

#### Option 1: a GitHub token

The user creates a token that can manage the organization's self-hosted runners: a fine grained token with **Organization permissions › Self-hosted runners: Read and write**, or a classic token with the `admin:org` scope. If the GitHub CLI is signed in with `admin:org` (`gh auth refresh -s admin:org`), `gh auth token` prints a usable token. Put it in the config as `auth: { pat_token: <token> }` (see Configure). When the token expires, runners stop until it is replaced.

#### Option 2: your own GitHub App

The user creates it; you only write the config afterwards.

1. `https://github.com/organizations/<org>/settings/apps` → **New GitHub App**. Webhook: uncheck **Active**. Permission: **Organization permissions › Self-hosted runners: Read and write**.
2. Generate a private key and save the `.pem` file on this Mac, for example `~/.elastic-fruit-runner/github-app-key.pem`.
3. **Install App** on the organization. The installation ID is the number at the end of the installation URL.
4. Copy the **Client ID** from the App settings page.

Ask the user for the Client ID, the installation ID and the key file path. Do not ask for the key contents.

### Configure

Write `~/.elastic-fruit-runner/config.yaml`. This example uses option 2; for option 1 replace the `github_app` block with `pat_token: <token>`:

```yaml
orgs:
  - org: your-org
    auth:
      github_app:
        client_id: Iv1.xxxxxxxxxxxxxxxx
        installation_id: 12345678
        private_key_path: /Users/you/.elastic-fruit-runner/github-app-key.pem
    runner_group: Default
    runner_sets:
      - name: efr-macos-arm64
        backend: tart
        image: ghcr.io/cirruslabs/macos-tahoe-xcode:26.3
        labels: [self-hosted, macos, arm64]
        max_runners: 2
      - name: efr-linux-arm64
        backend: docker
        image: ghcr.io/quipper/actions-runner:2.337.0
        labels: [self-hosted, linux, arm64]
        max_runners: 4
        platform: linux/arm64
      - name: efr-linux-amd64
        backend: docker
        image: ghcr.io/quipper/actions-runner:2.337.0
        labels: [self-hosted, linux, x64]
        max_runners: 4
        platform: linux/amd64

idle_timeout: 15m
log_level: info
```

Keep only the runner sets the user needs. macOS allows at most 2 macOS VMs per Mac, so keep the Tart `max_runners` at 2 or less. The first macOS job pulls the VM image, which takes a while.

### Before any restart

Restarting or stopping the service cancels runners that are starting and removes running ones, on **every** runner set, Docker and Tart alike. Before `brew services restart` and before an upgrade:

1. Right before the restart, open or refresh **Runner Sets** in the Console (http://127.0.0.1:8080). Every runner set must show no runner in `preparing` or `busy`. The daemon log is not a safe substitute: a set can show zero runners there while new ones are about to start.

2. If any runner is preparing or busy, wait for it to finish, or ask the user. If you cannot read the current state, do not restart; ask the user.

### Start and check

```bash
brew services start elastic-fruit-runner
brew services info elastic-fruit-runner          # Running: true
grep '"msg":"scale set ready"' /opt/homebrew/var/log/elastic-fruit-runner.log | tail
grep '"msg":"listening for jobs"' /opt/homebrew/var/log/elastic-fruit-runner.log | tail
```

The local Console is at http://127.0.0.1:8080. The user chooses its admin password on first visit. After a config change, check [Before any restart](#before-any-restart), then run `brew services restart elastic-fruit-runner`.

### Use it in a workflow

`runs-on` is the runner set **name**:

```yaml
jobs:
  test:
    runs-on: efr-macos-arm64
    steps:
      - run: sw_vers && uname -m
```

### Watch jobs

Open **Jobs** in the Console (http://127.0.0.1:8080) to see each job's repository, workflow, runner set, start and end times, runner log and resource data, with a link back to GitHub Actions. On GitHub, `gh run view <run-id> --repo owner/name --log` reads a run's log.

### Runner errors

| Symptom | Cause | Fix |
|---|---|---|
| Log shows a config field error at start | Config is not valid | Fix the named field, then restart the service (see Before any restart) |
| `The number of VMs exceeds the system limit` in the log | Two macOS VMs already run on this Mac | Since v0.4.2 the job waits up to 6 minutes for a free slot, then fails. Lower parallel macOS jobs or add a Mac |
| Jobs stay queued after a repository became public | The runner group does not allow public repositories | Organization Settings › Actions › Runner groups › allow public repositories |
| Linux runner does not start, `docker-credential-osxkeychain` missing | Docker credential helper is not on the service PATH | Make sure it is in `/opt/homebrew/bin` or `/usr/local/bin` |
| `start runner failed` in the log | Backend could not start the runner | Read the lines around it in `/opt/homebrew/var/log/elastic-fruit-runner.log` |

Docs: https://elastic-fruit-runner.pages.dev/tutorials/getting-started/ · Configuration: https://elastic-fruit-runner.pages.dev/reference/configuration/
