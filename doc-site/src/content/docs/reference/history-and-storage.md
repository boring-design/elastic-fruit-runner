---
title: History and Storage Reference
description: Job logs, resource samples, accuracy, retention, and storage cleanup rules.
---

Job history, job logs, job resource samples, host samples, auth data, and config revisions use the configured SQLite database.

## Collection intervals

| Data | Interval |
|---|---|
| Running job log | Collected every two seconds |
| Running job resource sample | Collected every five seconds |
| Host resource sample | Collected every five seconds |
| Host rollup | One row per minute |
| Disk config state | Checked every two seconds |

## Job history

A job record can contain:

* GitHub job ID and display name
* Owner and repository
* Workflow reference and run ID
* Event and labels
* Runner set, runner, and backend
* Result
* Queued, Scale Set assigned, runner assigned, started, and completed times
* GitHub Actions URL

Jobs are ordered from newest to oldest. The normal UI page size is 50. The storage layer accepts at most 200 jobs in one page request.

## Job logs

Logs are stored as ordered chunks with a sequence number and recorded time.

The Console requests up to 500 chunks at a time. A sequence cursor prevents previously read chunks from being added twice.

Job logs describe the runner process. GitHub Actions step output remains in GitHub.

## Resource fields

A resource sample can contain:

* CPU percent
* Memory used and available
* Disk used and available
* Disk read and write bytes
* Network receive and send bytes
* One minute load
* Temperature

Not every source provides every field.

## Accuracy

| Source | Accuracy | Meaning |
|---|---|---|
| Docker container stats | Exact | Values reported for the runner container |
| Tart allocation | Estimate | VM memory and disk allocation, not exact guest use |
| Host collector | Exact for the host source | Whole host values, not one runner |

The UI marks estimated job data as **Tart host estimate**.

## Host retention

Five second host samples remain for 24 hours.

One minute host rollups remain for 30 days.

Queries inside the most recent 24 hours use five second samples. Older queries use one minute rollups.

The System page reports the earliest host sample still available.

## Job retention

Completed job records, job logs, and job resource samples remain for up to 30 days.

Running job data is not removed by retention cleanup.

## Storage limit

The SQLite database, its write ahead log, and its shared memory file are measured together.

Cleanup runs after each minute rollup:

1. Remove data older than 30 days.
2. Measure current database storage.
3. If storage is above 10 GiB, remove the oldest completed job data.
4. Compact and measure again.
5. Stop when storage is at or below 9 GiB or no completed job can be removed.

Running jobs are never selected by the size cleanup.

The daemon log file is shown separately on the System page. It is not part of the history database limit.

## Storage paths

`db_path` selects the SQLite file. The default is:

```text
~/.elastic-fruit-runner/jobs.db
```

The database file is set to mode `0600`.

`log_path` optionally writes daemon JSON logs to a file. Empty `log_path` sends logs to standard output.

See [Configuration Reference](/reference/configuration/) for path validation rules.
