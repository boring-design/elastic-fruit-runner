---
title: CLI Reference
description: Commands and flags supported by the Elastic Fruit Runner binary.
---

## Run the daemon

```text
elastic-fruit-runner [--config PATH]
```

The daemon loads config, starts runner controllers, starts the Console, and waits for jobs.

| Flag | Default | Description |
|---|---|---|
| `--config PATH` | Config search paths | Select one YAML config file |
| `--help` | | Show usage for the command |

The environment variable `ELASTIC_FRUIT_RUNNER_CONFIG` is an alternative to `--config`. When both are set, the flag wins.

Without `--config` or the environment variable, see [Configuration Reference](/reference/configuration/) for the search order.

Example:

```sh
elastic-fruit-runner --config /etc/elastic-fruit-runner/config.yaml
```

## Reset the Console password

```text
elastic-fruit-runner reset-password [--config PATH]
```

The command opens the configured SQLite database, removes the local admin password, and removes every Console session.

Stop the daemon before running this command. Start it again, then create a new admin password in the Console.

Example:

```sh
elastic-fruit-runner reset-password --config /etc/elastic-fruit-runner/config.yaml
```

See [How to reset the Console password](/how-to/reset-console-password/) for service specific steps.

## Exit behavior

The daemon listens for `SIGINT` and `SIGTERM`. It stops the HTTP server and runner controllers before exit.

Startup errors are written as JSON logs and return a nonzero exit status.
