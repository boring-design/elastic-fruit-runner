# elastic-fruit-runner

Elastic GitHub Actions self-hosted runner manager for Apple Silicon.

- **Tart mode** — ephemeral macOS VMs via [Tart](https://tart.run), one per job, auto-scaled
- **Linux arm64 / amd64** via Docker (Docker-in-Docker)
- Powered by the official [GitHub Runner Scale Set Client](https://github.com/actions/scaleset) (Go)

> **Status:** PoC — core flow works, not production-hardened yet.

---

## Getting Started

See the [documentation site](https://elastic-fruit-runner.pages.dev) for full guides:

- [Getting Started Tutorial](https://elastic-fruit-runner.pages.dev/tutorials/getting-started/)
- [macOS Installation](https://elastic-fruit-runner.pages.dev/how-to/install-macos/)
- [Linux Deployment (Docker)](https://elastic-fruit-runner.pages.dev/how-to/install-linux-docker/)
- [Configuration Reference](https://elastic-fruit-runner.pages.dev/reference/configuration/)
- [GitHub App Auth](https://elastic-fruit-runner.pages.dev/how-to/configure-github-app/)
- [CLI Reference](https://elastic-fruit-runner.pages.dev/reference/cli/)
- [How it works](https://elastic-fruit-runner.pages.dev/explanation/what-is-elastic-fruit-runner/)

---

## Development

```sh
# Show all available targets
make help

# Build the binary (output in output/)
make build

# Run tests
make test

# Quick local check before committing (format, vet, build)
make check
```

---

## Roadmap

- [x] Linux arm64 runner (Docker)
- [x] Linux amd64 runner (Docker + Rosetta 2)
- [x] GitHub App auth
- [ ] Warm pool (pre-clone VMs to reduce job start latency)
- [ ] Wails GUI dashboard
