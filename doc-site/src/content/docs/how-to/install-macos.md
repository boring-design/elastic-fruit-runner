---
title: How to install on macOS
description: Install and run elastic-fruit-runner on macOS with Homebrew.
---

## Install via Homebrew

```sh
brew install boring-design/tap/elastic-fruit-runner
```

### Build from source

```sh
git clone https://github.com/boring-design/elastic-fruit-runner
cd elastic-fruit-runner
make build
```

The binary will be in `output/elastic-fruit-runner`.

## Configure

Create `~/.elastic-fruit-runner/config.yaml`. See the [configuration reference](/reference/configuration/) for all options.

```sh
mkdir -p ~/.elastic-fruit-runner
```

A minimal example:

```yaml
orgs:
  - org: your-org
    auth:
      pat_token: ghp_xxx
    runner_group: Default
    runner_sets:
      # macOS runners via Tart VMs
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

      # Linux amd64 runners via Docker (Rosetta 2 emulation)
      - name: efr-linux-amd64
        backend: docker
        image: ghcr.io/actions-runner-controller/actions-runner-controller/actions-runner-dind:latest
        labels: [self-hosted, linux, amd64]
        max_runners: 4
        platform: linux/amd64

idle_timeout: 15m
```

## Prevent sleep (headless / always-on servers)

If the Mac is used as a dedicated runner host (e.g. a Mac mini with the lid closed or no display), you should disable sleep and disk hibernation so that Docker and Tart stay responsive after long idle periods.

```sh
# Disable system sleep
sudo pmset -a sleep 0

# Disable disk sleep (prevents Docker daemon from hanging after wake)
sudo pmset -a disksleep 0

# Disable hibernation (no memory-to-disk writes)
sudo pmset -a hibernatemode 0

# Disable standby (prevents deep sleep on battery-capable Macs)
sudo pmset -a standby 0
```

Verify with:

```sh
sudo pmset -g
```

The key values should all be `0`:

| Setting         | Value | Effect                              |
|-----------------|-------|-------------------------------------|
| `sleep`         | 0     | Never enter system sleep            |
| `disksleep`     | 0     | Keep disks spinning                 |
| `hibernatemode` | 0     | No memory dump to disk              |
| `standby`       | 0     | No deep sleep after prolonged idle  |

:::caution
Without these settings, macOS will hibernate after the lid is closed or the machine is idle for a long time. On wake, the Docker daemon may become unresponsive, causing Docker-backed runner sets to silently hang during cleanup.
:::

## Run as a service (recommended)

```sh
brew services start elastic-fruit-runner
```

Check status:

```sh
brew services info elastic-fruit-runner
```

Stop:

```sh
brew services stop elastic-fruit-runner
```

## Run manually (foreground)

```sh
elastic-fruit-runner
```

With a specific config file:

```sh
elastic-fruit-runner --config /path/to/config.yaml
```

## View logs

Logs are written to the Homebrew log directory when running as a service:

```sh
tail -f /opt/homebrew/var/log/elastic-fruit-runner.log
```

On Intel Macs:

```sh
tail -f /usr/local/var/log/elastic-fruit-runner.log
```

Logs are JSON-formatted. Use `jq` for filtering:

```sh
# Show only errors
tail -f /opt/homebrew/var/log/elastic-fruit-runner.log | jq 'select(.level == "ERROR")'

# Show runner set activity
tail -f /opt/homebrew/var/log/elastic-fruit-runner.log | jq 'select(.runnerSet != null)'
```

When running manually, logs go to stdout.

## Update

```sh
brew upgrade elastic-fruit-runner
brew services restart elastic-fruit-runner
```

## Uninstall

```sh
brew services stop elastic-fruit-runner
brew uninstall elastic-fruit-runner
rm -rf ~/.elastic-fruit-runner
```
