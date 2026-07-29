---
title: How to upgrade Elastic Fruit Runner
description: Upgrade a Homebrew or Docker installation and confirm that the service is healthy.
---

## Check the current state

Open the Console before the upgrade.

1. Open **Jobs** and wait for running jobs to finish.
2. Open **Config** and confirm that the disk config is valid.
3. Note the active config hash.

Restarting the service can interrupt a running job.

## Upgrade a Homebrew installation

```sh
brew update
brew upgrade elastic-fruit-runner
brew services restart elastic-fruit-runner
```

Check that the service is running:

```sh
brew services info elastic-fruit-runner
```

## Upgrade a Docker Compose installation

```sh
docker compose pull elastic-fruit-runner
docker compose up -d elastic-fruit-runner
```

Check the new container:

```sh
docker compose ps elastic-fruit-runner
docker compose logs --tail 100 elastic-fruit-runner
```

## Verify the upgrade

Open the Console and check:

1. **Overview** shows a connected daemon.
2. **Config** shows the expected active and disk hashes.
3. **Runner Sets** shows each configured set.
4. **Jobs** accepts new work.
5. **System** shows new host samples.

If the disk config is not valid after the upgrade, follow [the Console guide](/how-to/use-console/) before starting new jobs.
