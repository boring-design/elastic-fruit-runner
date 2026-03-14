# elastic-fruit-runner

Elastic GitHub Actions self-hosted runner manager for Apple Silicon.

- **Host mode** — run the GitHub Actions runner directly on the machine, no VM overhead
- **Tart mode** — ephemeral macOS VMs via [Tart](https://tart.run), one per job, auto-scaled
- **Linux arm64 / amd64** via Docker _(planned)_
- Powered by the official [GitHub Runner Scale Set Client](https://github.com/actions/scaleset) (Go)

> **Status:** PoC — core flow works, not production-hardened yet.

---

## Quick Start

### Prerequisites

- Go 1.24+
- A GitHub PAT with `manage_runners:org` scope (org-level) or `repo` scope (repo-level)
- **Tart mode only:** Apple Silicon Mac + [Tart](https://tart.run) installed (`brew install cirruslabs/cli/tart`) + a base VM image pulled locally:
  ```sh
  tart pull ghcr.io/cirruslabs/macos-sequoia-base:latest
  ```

### Build

```sh
git clone https://github.com/boring-design/elastic-fruit-runner
cd elastic-fruit-runner
go build -o elastic-fruit-runner ./cmd/daemon
```

### Run

#### Host mode (simplest — no VM, runner runs directly on your machine)

```sh
GITHUB_TOKEN=ghp_xxx \
GITHUB_CONFIG_URL=https://github.com/your-org \
./elastic-fruit-runner --mode host
```

Each job gets an isolated work directory under `~/.elastic-fruit-runner/work/`, which is cleaned up after the job completes. The runner binary is cached in `~/.elastic-fruit-runner/runner/`.

#### Tart mode (ephemeral VMs — full isolation)

```sh
GITHUB_TOKEN=ghp_xxx \
GITHUB_CONFIG_URL=https://github.com/your-org \
./elastic-fruit-runner --mode tart
```

Each job runs in a fresh Tart VM cloned from the base image, destroyed after completion.

#### Auth — GitHub App (recommended for org deployments)

Create a GitHub App with **Organization > Self-hosted runners: Read and write** permission, install it on your org, then:

```sh
GITHUB_APP_CLIENT_ID=Iv1.xxxxxxxxxxxxxxxx \
GITHUB_APP_INSTALLATION_ID=12345678 \
GITHUB_APP_PRIVATE_KEY_PATH=/path/to/private-key.pem \
GITHUB_CONFIG_URL=https://github.com/your-org \
./elastic-fruit-runner --mode host
```

#### Auth — Personal Access Token

```sh
GITHUB_TOKEN=ghp_xxx \
GITHUB_CONFIG_URL=https://github.com/your-org \
./elastic-fruit-runner --mode host
```

> Scope required: `manage_runners:org` (org-level) or `repo` (repo-level).

**Repo-level runner** (only one repo):
```sh
GITHUB_TOKEN=ghp_xxx \
GITHUB_CONFIG_URL=https://github.com/your-org/your-repo \
./elastic-fruit-runner --mode host
```

**All flags:**
```
  # Backend mode
  --mode                  'host' (run on host directly) or 'tart' (ephemeral VMs) (default: tart)

  # Auth — GitHub App (takes precedence over PAT if set)
  --app-client-id         GitHub App Client ID (or GITHUB_APP_CLIENT_ID env)
  --app-installation-id   GitHub App Installation ID (or GITHUB_APP_INSTALLATION_ID env)
  --app-private-key       Path to PEM private key file (or GITHUB_APP_PRIVATE_KEY_PATH env)

  # Auth — PAT (fallback)
  --token                 GitHub PAT (or GITHUB_TOKEN env)

  # Common
  --url                   GitHub config URL — org or repo (or GITHUB_CONFIG_URL env)
  --runner-group          Runner group name (default: Default)
  --scale-set-name        Name shown in GitHub Actions UI (default: elastic-fruit-runner)
  --max-runners           Max concurrent runners (default: 2)

  # Tart mode only
  --vm-image              Tart base image to clone per job (default: ghcr.io/cirruslabs/macos-sequoia-base:latest)
```

### Use in a workflow

```yaml
jobs:
  build:
    runs-on: elastic-fruit-runner   # matches --scale-set-name
    steps:
      - uses: actions/checkout@v4
      - run: sw_vers   # runs inside ephemeral macOS VM
```

---

## How it works

```
GitHub Actions → Scale Set API (long-poll) → Daemon → Backend (host or tart) → ephemeral runner → job → cleanup
```

1. Daemon registers as a Runner Scale Set and long-polls GitHub for job signals
2. Job arrives → backend prepares environment (host: create work dir / tart: clone + start VM) → inject JIT runner config → run job
3. Job completes → runner exits → backend cleans up (host: remove work dir / tart: destroy VM)

See [ARCHITECTURE.md](ARCHITECTURE.md) for details.

---

## Roadmap

- [ ] Linux arm64 runner (Docker / OrbStack)
- [ ] Linux amd64 runner (Docker + Rosetta 2)
- [ ] GitHub App auth (preferred over PAT for org deployments)
- [ ] Warm pool (pre-clone VMs to reduce job start latency)
- [ ] Wails GUI dashboard
