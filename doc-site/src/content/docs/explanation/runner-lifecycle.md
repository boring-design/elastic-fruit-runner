---
title: Runner lifecycle
description: How GitHub assignments become short lived Docker containers or Tart virtual machines.
---

Elastic Fruit Runner separates the GitHub control flow from the backend resource flow.

GitHub decides that a job belongs to a runner set. The daemon decides how many local runners to prepare, up to the configured capacity. Docker or Tart then creates the environment that runs the job.

## Scope and Scale Set

Each configured organization or repository is a GitHub scope. A runner set inside that scope defines labels, capacity, and a backend image.

The daemon creates a Scale Set listener for each runner set. The listener receives the number of assigned jobs from GitHub. That number includes jobs waiting for a runner and jobs already running.

The daemon compares assigned jobs with runners that are preparing, idle, or busy. It creates only the missing number and never goes above `max_runners`. Counting preparing runners prevents two updates from creating the same capacity twice.

## Preparing

A new runner first enters `preparing`.

The daemon asks GitHub for a Just in Time runner config. It then creates a Docker container or Tart virtual machine and starts the GitHub Actions runner inside it.

Preparation can include pulling an image, cloning and starting a virtual machine, or starting a container. The runner becomes idle only after the backend has started it.

If preparation fails, the daemon removes the local resource and tries to remove the GitHub runner record. This cleanup is best effort because either side may already be unavailable.

## Idle and running

An idle runner waits for GitHub to assign one job.

When GitHub reports that the job started, the daemon marks the runner as busy and records the job. While it runs, the daemon collects runner logs every two seconds and backend resource data every five seconds.

The Just in Time config limits the runner to one job. It does not return to a shared idle pool after that job.

An idle runner that receives no work before `idle_timeout` is also removed. This keeps unused containers and virtual machines from staying on the host.

## Completed and cleanup

When GitHub reports completion, the daemon records the result, marks the runner done, and starts backend cleanup.

Docker cleanup removes the container. Tart cleanup stops and deletes the virtual machine. The job record, captured runner log, and resource samples remain in local history until retention removes them.

This design separates two different facts:

* GitHub job state describes the work.
* Runner state describes the local environment that can accept or run work.

A completed job can remain in history even though its runner no longer exists.

## Shutdown

During daemon shutdown, new preparation is canceled. Idle and busy local resources are cleaned up and their GitHub runner records are removed when possible.

A service restart can therefore interrupt a running job. This is why config changes require an operator to choose the restart time instead of letting the Console restart the daemon.

See [How to check runner capacity](/how-to/check-runner-capacity/) for the live states and [History and Storage Reference](/reference/history-and-storage/) for recorded data.
