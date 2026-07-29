---
title: How to reset the Console password
description: Clear the local Console admin and create a new password.
---

Reset removes the current admin password and every Console session. It does not remove runner config, jobs, logs, or resource history.

## Homebrew

Stop the daemon:

```sh
brew services stop elastic-fruit-runner
```

Reset the admin:

```sh
elastic-fruit-runner reset-password --config ~/.elastic-fruit-runner/config.yaml
```

Start the daemon:

```sh
brew services start elastic-fruit-runner
```

## Docker Compose

Stop the daemon:

```sh
docker compose stop elastic-fruit-runner
```

Run the reset command with the same config and data volumes:

```sh
docker compose run --rm elastic-fruit-runner reset-password --config /etc/elastic-fruit-runner/config.yaml
```

Start the daemon:

```sh
docker compose up -d elastic-fruit-runner
```

## systemd

Stop the daemon:

```sh
sudo systemctl stop elastic-fruit-runner
```

Run the reset command with the service config:

```sh
sudo elastic-fruit-runner reset-password --config /etc/elastic-fruit-runner/config.yaml
```

Start the daemon:

```sh
sudo systemctl start elastic-fruit-runner
```

## Create the new password

Read the new setup code from the local daemon log. Then follow [How to set up the Console](/how-to/set-up-console/).

Old browser sessions no longer work after the reset.
