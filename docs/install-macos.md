# macOS Installation Guide

## Install

```sh
brew install boring-design/tap/elastic-fruit-runner
```

<details>
<summary>Build from source</summary>

```sh
git clone https://github.com/boring-design/elastic-fruit-runner
cd elastic-fruit-runner
go build ./cmd/elastic-fruit-runner
```

</details>

## Configure

Create a config file at `~/.elastic-fruit-runner/config.yaml`:

```sh
mkdir -p ~/.elastic-fruit-runner
cp "$(brew --prefix)/share/elastic-fruit-runner/config.example.yaml" ~/.elastic-fruit-runner/config.yaml
# or copy from the repo: cp config.example.yaml ~/.elastic-fruit-runner/config.yaml
```

Edit the file:

```yaml
github:
  url: https://github.com/your-org
  token: ghp_xxx

runner_sets:
  - name: efr-macos-arm64
    backend: tart
    image: ghcr.io/cirruslabs/macos-sequoia-base:latest
    labels: [self-hosted, macOS, ARM64]
    max_runners: 2
```

See [config.example.yaml](../config.example.yaml) for all options.

### GitHub App auth (recommended for orgs)

```yaml
github:
  url: https://github.com/your-org
  app:
    client_id: Iv1.xxxxxxxxxxxxxxxx
    installation_id: 12345678
    private_key_path: /path/to/private-key.pem
```

### Secrets via environment

If you prefer not to put tokens in the YAML file, use an env file instead. Environment variables override config file values:

```sh
cat > ~/.elastic-fruit-runner/env << 'EOF'
GITHUB_TOKEN=ghp_xxx
EOF
```

## Run as a service (recommended)

Start the daemon and register it to auto-start on login:

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
# or with a specific config:
elastic-fruit-runner --config /path/to/config.yaml
# or with env var overrides:
GITHUB_TOKEN=ghp_xxx GITHUB_CONFIG_URL=https://github.com/your-org elastic-fruit-runner
```

## Viewing logs

Logs are written to the Homebrew log directory when running as a service:

```sh
# Follow logs in real time
tail -f /opt/homebrew/var/log/elastic-fruit-runner.log

# Or on Intel Macs:
tail -f /usr/local/var/log/elastic-fruit-runner.log
```

Logs are JSON-formatted (one JSON object per line), so you can use `jq` for filtering:

```sh
# Show only errors
tail -f /opt/homebrew/var/log/elastic-fruit-runner.log | jq 'select(.level == "ERROR")'

# Show runner set activity
tail -f /opt/homebrew/var/log/elastic-fruit-runner.log | jq 'select(.runnerSet != null)'
```

When running manually (not via `brew services`), logs go to stdout.

## Updating

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
