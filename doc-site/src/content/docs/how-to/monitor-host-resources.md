---
title: How to monitor host resources
description: Use the System page to check runtime, storage, current host use, and resource history.
---

Open **System** in the Console.

## Check runtime

The Runtime panel shows:

* Elastic Fruit Runner version
* Go version
* Operating system and architecture
* Start time
* Uptime

Use these values when reporting a problem or confirming an upgrade.

If a value is **Unknown**, check the local daemon log for build or startup errors.

## Check storage

The Storage panel shows:

* Database path and current size
* Log path and current size
* Config path

If the log path says **Standard output**, use the log source managed by Homebrew, Docker, or systemd.

See [History and Storage Reference](/reference/history-and-storage/) for cleanup order and storage limits.

## Check current resources

The current sample shows:

* CPU use
* Memory use
* Disk use
* Temperature when the host provides it

Temperature is optional. **Temperature is unavailable on this system** is a supported state.

## Check history

Choose one range:

* Last hour
* Last 24 hours
* Last 7 days
* Last 30 days

The page shows CPU, memory, disk read, disk write, and temperature history when enough samples exist.

**Available since** reports the earliest host sample still stored. A range can start before that time, but the chart cannot show removed data.

If a chart says that no history exists yet, wait for at least two samples. The daemon records a host sample every five seconds.

If the whole history request fails, check database access and the daemon log.

## Compare host and job data

Use **System** to see pressure on the whole host. Use job detail in **Jobs** to see data collected for one runner.

Host data cannot prove that one Tart guest used a specific amount of CPU or memory. Tart job values are marked as estimates when guest data is unavailable.
