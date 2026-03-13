# elastic-fruit-runner

Elastic GitHub Actions self-hosted runner manager for Apple Silicon.

- **macOS runner** via [Tart](https://tart.run) — ephemeral VMs, one per job, auto-scaled
- **Linux arm64 / amd64** via Docker _(planned)_
- Powered by the official [GitHub Runner Scale Set Client](https://github.com/actions/scaleset) (Go)

> **Status:** PoC — core flow works, not production-hardened yet.

---

## Quick Start

### Prerequisites

- Apple Silicon Mac (M1/M2/M3)
- [Tart](https://tart.run) installed: `brew install cirruslabs/cli/tart`
- A Tart base VM image pulled locally, e.g.:
  ```sh
  tart pull ghcr.io/cirruslabs/macos-sequoia-base:latest
  ```
- Go 1.24+
- A GitHub PAT with `manage_runners:org` scope (org-level) or `repo` scope (repo-level)

### Build

```sh
git clone https://github.com/boring-design/elastic-fruit-runner
cd elastic-fruit-runner
go build -o elastic-fruit-runner ./cmd/daemon
```

### Run

**Org-level runner** (all repos in the org can use it):
```sh
GITHUB_TOKEN=ghp_xxx \
GITHUB_CONFIG_URL=https://github.com/your-org \
./elastic-fruit-runner
```

**Repo-level runner** (only one repo):
```sh
GITHUB_TOKEN=ghp_xxx \
GITHUB_CONFIG_URL=https://github.com/your-org/your-repo \
./elastic-fruit-runner
```

**All flags:**
```
  --token           GitHub PAT (or GITHUB_TOKEN env)
  --url             GitHub config URL — org or repo (or GITHUB_CONFIG_URL env)
  --runner-group    Runner group name (default: Default)
  --scale-set-name  Name shown in GitHub Actions UI (default: elastic-fruit-runner)
  --vm-image        Tart base image to clone per job (default: ghcr.io/cirruslabs/macos-sequoia-base:latest)
  --max-runners     Max concurrent VMs, Apple EULA caps macOS VMs at 2 per host (default: 2)
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
GitHub Actions → Scale Set API (long-poll) → Daemon → tart clone → VM → ephemeral runner → job → tart delete
```

1. Daemon registers as a Runner Scale Set and long-polls GitHub for job signals
2. Job arrives → clone base Tart VM → start VM → inject JIT runner config → run job
3. Job completes → runner self-deregisters → VM destroyed
4. Max 2 concurrent VMs (Apple EULA limit for macOS virtualisation on a single host)

See [ARCHITECTURE.md](ARCHITECTURE.md) for details.

---

## Roadmap

- [ ] Linux arm64 runner (Docker / OrbStack)
- [ ] Linux amd64 runner (Docker + Rosetta 2)
- [ ] GitHub App auth (preferred over PAT for org deployments)
- [ ] Warm pool (pre-clone VMs to reduce job start latency)
- [ ] Wails GUI dashboard
