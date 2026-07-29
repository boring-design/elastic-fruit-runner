---
title: How to set up the Console
description: Use the first start setup code to create the Console admin password.
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

## Find the setup code

The daemon creates a setup code only when no admin password exists. The code is valid only for the current daemon process.

For Homebrew:

```sh
grep '"msg":"console admin setup required"' /opt/homebrew/var/log/elastic-fruit-runner.log | tail -n 1
```

For Docker Compose:

```sh
docker compose logs elastic-fruit-runner | grep '"msg":"console admin setup required"' | tail -n 1
```

For systemd:

```sh
sudo journalctl -u elastic-fruit-runner | grep '"msg":"console admin setup required"' | tail -n 1
```

Treat the setup code as a password. Do not paste it into an issue, chat, screenshot, or shared log.

## Create the admin

1. Open [http://127.0.0.1:8080](http://127.0.0.1:8080).
2. Enter the setup code.
3. Enter an admin password with 12 to 256 characters.
4. Enter the password again.
5. Select **Create admin**.

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

If the setup code is missing or no longer valid, [reset the Console password](/how-to/reset-console-password/).
