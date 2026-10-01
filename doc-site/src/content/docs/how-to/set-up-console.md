---
title: How to set up the Console
description: Create the Console admin password on first start.
---

## Start Elastic Fruit Runner

Start the service with the method used for your installation.

For Homebrew:

```sh
brew services start elastic-fruit-runner
```

For Docker Compose:

```sh
docker compose up -d elastic-fruit-runner
```

For systemd:

```sh
sudo systemctl start elastic-fruit-runner
```

## Create the admin

The Console asks for a new admin password only when no admin password exists.

1. Open [http://127.0.0.1:8080](http://127.0.0.1:8080).
2. Enter an admin password.
3. Enter the password again.
4. Select **Create admin**.

The Console signs you in and opens **Overview**.

## Use the setup wizard

When the service starts without a config file, **Overview** shows the setup wizard and a **Setup** entry appears in the left navigation. The wizard writes the config file for you in six steps:

1. **Target**: choose an organization or a repository.
2. **Credentials**: enter a personal access token or GitHub App details. The page lists the scopes or permissions the credential needs.
3. **Runner sets**: pick from the presets the wizard suggests for this host and edit the name, image, labels, and limits.
4. **Test**: the wizard checks GitHub access and the backend tools. You can continue even when a check fails.
5. **Preview**: review the generated YAML. The token is hidden. **Open in advanced editor** moves the YAML to the **Config** page for manual edits.
6. **Save**: the wizard validates and writes the config file, then offers **Restart to apply**.

Click **Restart to apply** after the save. The daemon starts again with the new config and the page reloads. You can open the wizard again at any time from `#/setup`.

## Confirm access

Open each page in the left navigation:

1. Overview
2. Jobs
3. Runner Sets
4. Config
5. System

Sign out, then sign in again with the admin password.

Only one local admin account exists. A session lasts up to 24 hours.

If you lose the admin password, [reset the Console password](/how-to/reset-console-password/).
