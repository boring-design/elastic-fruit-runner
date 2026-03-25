---
title: CLI Reference
description: Command-line flags for elastic-fruit-runner.
---

## Usage

```
elastic-fruit-runner [flags]
```

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--config PATH` | (see search paths below) | Path to the YAML configuration file |

### Config file search paths

When `--config` is not specified, the following paths are searched in order:

1. `~/.elastic-fruit-runner/config.yaml`
2. `/opt/homebrew/var/elastic-fruit-runner/config.yaml`
3. `/usr/local/var/elastic-fruit-runner/config.yaml`
4. `/etc/elastic-fruit-runner/config.yaml`

## Examples

Run with default config file search:

```sh
elastic-fruit-runner
```

Run with a specific config file:

```sh
elastic-fruit-runner --config /path/to/config.yaml
```

Override log level via environment variable:

```sh
LOG_LEVEL=debug elastic-fruit-runner
```
