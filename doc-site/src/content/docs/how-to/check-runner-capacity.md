---
title: How to check runner capacity
description: Use the Runner Sets page to check GitHub connection, configured capacity, and active runner state.
---

Open **Runner Sets** in the Console.

## Check the configured set

Each card shows:

* Runner set name
* Docker or Tart backend
* Active runner count and maximum runner count
* GitHub organization or repository scope
* Image
* Labels
* GitHub connection

If the page says **No runner sets configured**, open **Config** and confirm that at least one organization or repository contains a runner set.

## Check GitHub connection

The card reports **Connected** after the runner set starts listening for work.

If it reports **Disconnected**:

1. Check the daemon log for an auth or Scale Set error.
2. Confirm the PAT or GitHub App settings.
3. Confirm the organization or repository name.
4. Confirm GitHub App installation permissions when App auth is used.

See [How to configure GitHub App authentication](/how-to/configure-github-app/) for the required settings.

## Check active runners

The card counts runners in three states:

* `preparing` means the backend is creating and registering the runner.
* `idle` means the runner is ready for a job.
* `busy` means the runner is running a job.

The runner table shows each runner name, current state, and the time it entered that state.

If the active count equals the maximum, new jobs wait until a runner finishes. Increase `max_runners` only after checking available CPU, memory, disk, and backend limits.

## Check the matching job

Open **Jobs** and filter by the runner set name. This connects capacity state with the jobs using that set.

If a runner stays in `preparing`, use [How to investigate a job](/how-to/investigate-jobs/) and [Troubleshooting](/how-to/troubleshooting/) to check the backend.
