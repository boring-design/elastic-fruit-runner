---
title: How to edit and activate config
description: Validate a config change, save it to disk, and restart the service to activate it.
---

Open **Config** in the Console.

![Config page showing active and disk identity, the editor, and recent revisions](/console-config.png)

## Check config state

The page reports one state:

* **In sync** means the active config and disk config match.
* **Restart required** means a valid disk config differs from the active config.
* **Disk config invalid** means the disk file cannot be used. The active config is unchanged.

The active config is the version loaded when the daemon started. The disk config is the current file content. Editing or saving the disk file does not change running controllers.

## Edit the disk config

1. Edit YAML in **Disk config editor**.
2. Select **Validate**.
3. Fix every error.
4. Review every warning.

Validation errors include a config path and a reason. A config with errors cannot be saved.

GitHub connectivity is a warning because validation does not make a network request.

## Save the config

Select **Save to disk**.

If warnings exist, review them and select **Save with warnings**.

A successful save:

1. Replaces the config file in one atomic operation.
2. Sets file mode `0600`.
3. Creates a config revision.
4. Changes the page state to **Restart required**.

The Console hides PAT values and private key contents. Do not replace a masked secret unless you intend to change it.

## Check running jobs

Open **Jobs** before restart. Wait for running jobs to finish when possible.

A service restart can interrupt a running job.

## Restart the service

Use the command for your installation.

For Homebrew:

```sh
brew services restart elastic-fruit-runner
```

For Docker Compose:

```sh
docker compose restart elastic-fruit-runner
```

For systemd:

```sh
sudo systemctl restart elastic-fruit-runner
```

The Console does not restart the daemon and does not hot reload controllers.

## Confirm the active config

After restart:

1. Open **Config**.
2. Confirm the state is **In sync**.
3. Confirm active hash and disk hash match.
4. Open **Runner Sets** and confirm each expected set exists.
5. Open **Overview** and confirm GitHub connection state.

If the disk config is invalid, follow [How to recover config](/how-to/recover-config/).
