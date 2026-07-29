---
title: Console Reference
description: Console pages, refresh behavior, config states, and security properties.
---

The Console is embedded in the daemon and manages one daemon on one host.

The default address is `http://127.0.0.1:8080`.

## Pages

| Page | Data |
|---|---|
| Overview | Daemon uptime, GitHub connection, runner counts, job counts, config state, and current host use |
| Jobs | Stored jobs, filters, pagination, detail, logs, resource samples, and GitHub Actions links |
| Runner Sets | Configured scope, backend, image, labels, capacity, GitHub connection, and live runners |
| Config | Active and disk identity, strict validation, editor, diff, restart commands, and ten revisions |
| System | Build, platform, storage paths, storage size, current host use, and host history |

## Normal refresh

Overview, Jobs, Runner Sets, Config, System, and host history refresh every five seconds.

The daemon checks the disk config every two seconds.

A running job log requests new chunks every two seconds. A running job resource chart refreshes every five seconds.

## Job filters

Jobs supports:

* Result
* Runner set
* Repository
* Workflow
* One hour, 24 hour, 7 day, or 30 day range

The UI requests 50 jobs per page and uses a cursor for the next page.

## Runner states

| State | Meaning |
|---|---|
| `preparing` | Backend creation and runner registration are in progress |
| `idle` | Runner is ready for work |
| `busy` | Runner is running a job |

Each runner set has its own GitHub connection state.

## Job results

The Console displays `running`, `success`, `failure`, or `canceled`.

Job detail can contain queued, Scale Set assigned, runner assigned, started, and completed times. A field is `Unavailable` when GitHub did not provide it.

## Config states

| State | Meaning |
|---|---|
| **In sync** | Disk config matches active config |
| **Restart required** | Valid disk config differs from active config |
| **Disk config invalid** | Disk file is not usable. Active config is unchanged |

Saving or restoring config writes disk only. The Console never restarts the service or hot reloads controllers.

The daemon stores the newest ten full config revisions.

## Admin and session

The Console supports one local admin.

* First setup requires a one time code from the local daemon log.
* Password length is 12 through 256 characters.
* Passwords are stored as bcrypt hashes.
* A session lasts 24 hours.
* Session cookies are HttpOnly and SameSite Strict.
* Write calls require the session and a CSRF token.
* Repeated failed sign in attempts are rate limited.
* Password reset removes every active session.

Config responses mask PAT values and do not return private key contents.

## Network boundary

The daemon does not provide TLS. Keep the default local address or place the service behind a trusted TLS reverse proxy.

Do not expose the Console directly to the public internet.

## Related guides

* [Use the operations Console](/how-to/use-console/)
* [How to edit and activate config](/how-to/edit-config/)
* [How to recover config](/how-to/recover-config/)
