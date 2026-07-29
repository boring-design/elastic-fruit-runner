---
title: Console design
description: Why the Console is local, uses manual config activation, and keeps bounded history.
---

The Console is part of the Elastic Fruit Runner daemon. It is designed to operate one daemon on one host.

This keeps setup small and lets the Console show the same runner state, job records, config state, and host data that the daemon already owns. It is not a shared control plane for a fleet.

## Active config and disk config

The active config is the copy loaded when the daemon started. The disk config is the current YAML file.

Keeping these as separate identities makes changes visible without claiming that they are already active. The Console reports whether they match, whether a valid disk change needs a restart, or whether the disk file is invalid.

Saving does not replace live controllers. Changing a scope, auth method, backend, image, or capacity can affect running jobs and GitHub connections. An automatic restart would choose an interruption time for the operator. The Console therefore writes the file and leaves activation to a manual service restart.

See [How to edit and activate config](/how-to/edit-config/) for the task and [Console Reference](/reference/console/) for the states.

## Revisions and config mode

Each successful save creates a full config revision. A config loaded at normal startup is also marked as an active revision.

If the disk file becomes invalid, the last active revision gives the daemon a known config that previously started. The daemon can keep runner service available while the operator repairs the disk file.

If there is no active revision, the daemon cannot safely start runner controllers. It enters config mode instead. The Console and config editor remain available, but runner management does not start. This gives the operator a recovery path without pretending that an unknown config is active.

Restoring a revision writes it to disk. It still needs a manual restart for the same reason as any other config change.

See [How to recover config](/how-to/recover-config/) for both recovery cases.

## Local single admin model

The Console has one local admin because it manages one local daemon. It does not provide teams, roles, or an identity provider.

The first start uses a one time setup code from the daemon log. The admin password is stored as a bcrypt hash. Sessions use an HttpOnly, SameSite Strict cookie, and write calls also need a CSRF token.

These controls protect the application session. They do not create a safe public network boundary. The daemon has no built in TLS, so the Console should stay local or sit behind a trusted TLS reverse proxy.

This model is simple for a local host, but it is not a replacement for a shared access system.

## Samples, estimates, and retention

Docker provides container stats, so Docker job charts describe the runner container. Tart currently provides virtual machine allocation from the host, not exact guest use, so the Console marks it as an estimate.

Keeping that label matters more than filling every chart. An estimate can still show configured memory and disk capacity, but it should not be read as guest demand.

Host data uses five second samples for recent investigation. One minute rollups keep longer trends with less storage. Raw data expires after 24 hours, while rollups and completed job history expire after 30 days.

The database also has a size limit and protects running jobs from cleanup. These bounds keep local history useful without allowing it to grow forever on the runner host.

See [History and Storage Reference](/reference/history-and-storage/) for the exact intervals, limits, and data sources.
