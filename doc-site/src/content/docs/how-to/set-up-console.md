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
