---
title: How to recover config
description: Restore a saved config revision or repair a daemon that started in config mode.
---

## Recover with a saved active revision

When disk config is invalid, the daemon tries to load the last active revision.

If recovery succeeds:

* Controllers start with the last active config.
* The Console reports **Disk config invalid**.
* The invalid disk file does not replace the active config.

To restore a revision:

1. Open **Config**.
2. Find a known good item under **Recent revisions**.
3. Select **Restore to disk**.
4. Validate the restored disk config.
5. Open **Jobs** and wait for running jobs to finish.
6. Restart the service manually.
7. Confirm the state is **In sync**.

Restore writes the selected revision to disk. It does not change running controllers and does not restart the service.

The daemon keeps the newest ten config revisions.

## Recover when no active revision exists

If disk config is invalid and no active revision exists, the daemon starts in config mode.

In config mode:

* Runner controllers do not start.
* The Console uses the default local address.
* Admin setup and sign in remain available.
* The Config page remains available.

To repair the config:

1. Open [http://127.0.0.1:8080](http://127.0.0.1:8080).
2. Set up or sign in to the Console.
3. Open **Config**.
4. Replace the invalid YAML with a complete config.
5. Select **Validate**.
6. Fix every error.
7. Save the config.
8. Restart the service manually.

After restart, confirm that **Runner Sets** contains the expected sets and **Overview** shows GitHub connection state.

## Recover without Console access

Edit the configured YAML file with a local text editor.

Run the daemon in the foreground to see validation output:

```sh
elastic-fruit-runner --config /path/to/config.yaml
```

The error includes the config path and reason. Fix the file, then start the normal service again.

Use [Configuration Reference](/reference/configuration/) to check fields and limits.
