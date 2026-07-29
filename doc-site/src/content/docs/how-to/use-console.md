---
title: Use the operations console
description: Set up, operate, and recover the Elastic Fruit Runner console.
---

The console is served by the daemon on `http://127.0.0.1:8080` by default. It manages one daemon and one host.

## First login

On the first start, the daemon writes a one time setup code to its local log. Open the console, enter that code, and set the admin password.

The password is stored as a strong hash. Login uses an HttpOnly session cookie. The console never returns PAT values or private key contents.

## Password recovery

Stop the daemon, clear the local admin password, and start the daemon again:

```sh
elastic-fruit-runner reset-password --config /path/to/config.yaml
```

The next start writes a new setup code to the local log. Existing console sessions are removed.

## Pages

| Page | Purpose |
|------|---------|
| Overview | Current daemon, GitHub, runner, job, config, and host state |
| Jobs | Job filters, GitHub Actions links, lifecycle times, logs, and resource history |
| Runner Sets | Scope, backend, image, labels, capacity, connection state, and active runners |
| Config | Active config, disk config, validation, editing, revisions, and restart commands |
| System | Runtime details, storage use, current host state, and host history |

Docker job data comes from container logs and Docker resource data.

Tart job logs come from the runner process in the VM. Guest resource use is not available without a guest agent, so the console labels Tart allocation and host side values as estimates.

## Edit config

The active config is the version loaded when the daemon started. The disk config is the current file content.

Saving follows this order:

1. Parse YAML.
2. Check unknown fields, duplicate keys, required values, ranges, names, auth choice, backend settings, paths, CORS, and private key PEM data.
3. Stop when any error exists.
4. Ask for confirmation when only warnings exist.
5. Replace the disk file in one atomic operation.
6. Set file permissions to `0600`.
7. Save a revision.
8. Show `Restart required`.

Saving does not reload controllers and does not restart the daemon.

Run the command that matches the installation:

```sh
brew services restart elastic-fruit-runner
docker compose restart elastic-fruit-runner
sudo systemctl restart elastic-fruit-runner
```

A restart can interrupt running jobs. Check the Jobs page before restarting.

## Config recovery

The daemon keeps the newest ten config revisions.

If the disk config is invalid and an active revision exists, the daemon starts with the last active revision. The Config page reports `Disk config invalid`.

If no active revision exists, the daemon starts in config mode on the default local address. Controllers do not start. Authentication and the Config page remain available.

Restoring a revision only writes it to disk. A manual restart is still required.

## Data history

Host data is collected every five seconds. Raw samples remain available for twenty four hours. One minute summaries remain available for thirty days.

Job records, job logs, job resource data, and host resource data share a ten GB limit. Cleanup removes data older than thirty days first. If storage is still above the limit, cleanup removes the oldest completed jobs until use is below nine GB. Running jobs are never removed.

The System page shows the earliest available host sample.

## Upgrade

Before an upgrade, confirm that the current disk config is valid and note the active hash.

After an upgrade:

1. Open Overview and confirm GitHub connection state.
2. Open Config and confirm the active and disk hashes.
3. Open Jobs and confirm new work appears.
4. Open System and confirm new host samples appear.

An existing valid config becomes the first active revision after controllers initialize successfully.

## Network access

The daemon does not provide TLS. Keep the console on a trusted local network or place it behind a TLS reverse proxy.

Every management RPC requires login. Config save, revision restore, and logout also require a valid CSRF token.
