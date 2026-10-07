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

## Enroll with Elastic Fruit Cloud

```text
elastic-fruit-runner enroll --server URL --token TOKEN [--max-runners N] [--config PATH]
```

The command registers this host with an Elastic Fruit Cloud server, writes the agent credential to `~/.elastic-fruit-runner/agent-credential`, and writes the `cloud` block into the config file.

| Flag | Default | Description |
|---|---|---|
| `--server URL` | required | Cloud server URL, `https://` unless the host is `localhost` or `127.0.0.1` |
| `--token TOKEN` | required | One time enrollment token from the cloud console |
| `--max-runners N` | `1` | Maximum number of runners on this host |
| `--config PATH` | Config search paths | Config file to update or create |

The command refuses a config file that still contains `orgs` or `repos`.

Example:

```sh
elastic-fruit-runner enroll --server https://cloud.example.com --token efc_abc123 --max-runners 2
```

See [How to run in cloud mode](/how-to/cloud-mode/) for the full flow.

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

## Print the version

```text
elastic-fruit-runner version
elastic-fruit-runner --version
```

Both forms print one line with the version, the Git commit, and the build date:

```text
elastic-fruit-runner 0.3.0 (0123456789abcdef0123456789abcdef01234567, 2026-10-06T08:00:00Z)
```

Release binaries get these values from the release build. A local `go build` prints `dev` as the version and reads the commit and date from the Git checkout when available.

## Exit behavior

The daemon listens for `SIGINT` and `SIGTERM`. It stops the HTTP server and runner controllers before exit.

Startup errors are written as JSON logs and return a nonzero exit status.
