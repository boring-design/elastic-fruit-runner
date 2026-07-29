---
title: How to investigate a job
description: Filter job history, read runner logs, inspect resource data, and open the matching GitHub Actions job.
---

## Find the job

Open **Jobs** in the Console.

Use one or more filters:

* Result
* Runner set
* Repository
* Workflow
* Time range

The time range can be one hour, 24 hours, 7 days, or 30 days. Each page contains up to 50 jobs. Use **Previous** and **Next** to move through stored history.

If the table says **No jobs recorded**, confirm that a configured runner has accepted at least one job.

If the table says **Job history unavailable**, check the daemon log and confirm that the configured database path is writable.

## Open job detail

Select a row to open the job detail panel.

Check:

1. Repository and workflow
2. Runner and backend
3. Queued time
4. Scale set assignment time
5. Runner assignment time
6. Start and completion time

These times show where a job spent time before its runner started work.

## Read the runner log

Use **Runner log** to inspect runner process output.

For a running job, the Console requests new log chunks every two seconds. A completed job stops polling after all stored chunks are read.

If the panel says **No log data is available**, the backend did not collect a runner log for that job. Check the daemon log for backend startup or collection errors.

## Inspect resource history

For a Docker job, the charts show container data:

* CPU
* Memory
* Disk read and write
* Network receive and send

For a Tart job without guest resource data, the panel shows **Tart host estimate**. Memory and disk values describe VM allocation. Host side values are estimates and do not describe exact guest use.

If a chart has fewer than two samples, the Console reports that no history is available yet.

See [History and Storage Reference](/reference/history-and-storage/) for data source, accuracy, and retention rules.

## Open GitHub Actions

Select **Open in GitHub Actions** to open the matching workflow run and job.

Use the Console log to inspect runner startup and local resource behavior. Use GitHub Actions for step output, annotations, artifacts, and workflow controls.
