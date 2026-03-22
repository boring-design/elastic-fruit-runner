# elastic-fruit-runner

Elastic GitHub Actions self-hosted runner manager for Apple Silicon.

- **Host mode** — run the GitHub Actions runner directly on the machine, no VM overhead
- **Tart mode** — ephemeral macOS VMs via [Tart](https://tart.run), one per job, auto-scaled
- **Linux arm64 / amd64** via Docker (Docker-in-Docker)
- Powered by the official [GitHub Runner Scale Set Client](https://github.com/actions/scaleset) (Go)

> **Status:** PoC — core flow works, not production-hardened yet.

---

## Installation

| Platform | Method |
|----------|--------|
| **macOS** | `brew install boring-design/tap/elastic-fruit-runner` |
| **Linux** | `docker run ghcr.io/boring-design/elastic-fruit-runner:latest` |

Detailed guides:
- [macOS Installation & Service Setup](docs/install-macos.md) — Homebrew install, `brew services`, log viewing
- [Linux Deployment (Docker)](docs/install-linux-docker.md) — Docker / Docker Compose / systemd

## Quick Start (macOS)

```sh
# 1. Install
brew install boring-design/tap/elastic-fruit-runner

# 2. Configure
mkdir -p ~/.elastic-fruit-runner
cat > ~/.elastic-fruit-runner/config.yaml << 'EOF'
github:
  url: https://github.com/your-org
  token: ghp_xxx

runner_sets:
  - name: efr-macos-arm64
    backend: tart
    image: ghcr.io/cirruslabs/macos-sequoia-base:latest
    labels: [self-hosted, macOS, ARM64]
    max_runners: 2
EOF

# 3. Start as a service (auto-starts on login)
brew services start elastic-fruit-runner

# 4. Check logs
tail -f /opt/homebrew/var/log/elastic-fruit-runner.log
```

### Use in a workflow

```yaml
jobs:
  build:
    runs-on: efr-macos-arm64
    steps:
      - uses: actions/checkout@v4
      - run: sw_vers
```

---

## Configuration

Configuration is loaded from (lowest to highest priority):

1. **Built-in defaults** — sensible out-of-the-box values
2. **Config file** — `~/.elastic-fruit-runner/config.yaml` (or `--config /path/to/file.yaml`)
3. **Env file** — `~/.elastic-fruit-runner/env` (KEY=VALUE, one per line)
4. **Environment variables** — `GITHUB_TOKEN`, `GITHUB_CONFIG_URL`, etc.
5. **CLI flags** — `--url`, `--token`

See [config.example.yaml](config.example.yaml) for a full annotated example.

### Auth — GitHub App (recommended for org deployments)

```yaml
github:
  url: https://github.com/your-org
  app:
    client_id: Iv1.xxxxxxxxxxxxxxxx
    installation_id: 12345678
    private_key_path: /path/to/private-key.pem
```

### Auth — Personal Access Token

```yaml
github:
  url: https://github.com/your-org
  token: ghp_xxx
```

> Scope required: **Organization > Self-hosted runners: Read and write** (org-level), or **Repository > Administration: Read and write** (repo-level).

### Environment variables

| Env var | Config file equivalent |
|---------|----------------------|
| `GITHUB_CONFIG_URL` | `github.url` |
| `GITHUB_TOKEN` | `github.token` |
| `GITHUB_APP_CLIENT_ID` | `github.app.client_id` |
| `GITHUB_APP_INSTALLATION_ID` | `github.app.installation_id` |
| `GITHUB_APP_PRIVATE_KEY_PATH` | `github.app.private_key_path` |
| `GITHUB_RUNNER_GROUP` | `runner_group` |

### CLI flags

```
  --config    Path to config file (default: ~/.elastic-fruit-runner/config.yaml)
  --url       GitHub config URL (overrides config file)
  --token     GitHub PAT (overrides config file)
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

- [x] Linux arm64 runner (Docker)
- [x] Linux amd64 runner (Docker + Rosetta 2)
- [x] GitHub App auth
- [ ] Warm pool (pre-clone VMs to reduce job start latency)
- [ ] Wails GUI dashboard
