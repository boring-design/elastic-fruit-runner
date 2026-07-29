---
title: Use the operations Console
description: Find the guide you need to set up, operate, and recover the Elastic Fruit Runner Console.
---

The Console is included in the Elastic Fruit Runner binary. It manages one daemon and one host.

Open [http://127.0.0.1:8080](http://127.0.0.1:8080) when you use the default listen address.

## Start here

* [Set up the Console](/how-to/set-up-console/)
* [Reset the Console password](/how-to/reset-console-password/)
* [Upgrade Elastic Fruit Runner](/how-to/upgrade/)

## Console pages

* **Overview** shows current daemon, runner, job, config, and host state.
* **Jobs** shows job history, filters, logs, resource data, and GitHub Actions links.
* **Runner Sets** shows scope, backend, image, labels, capacity, connection state, and active runners.
* **Config** shows active and disk config, validation, revisions, and restart instructions.
* **System** shows runtime details, storage use, current host state, and host history.

## Related reference

* [Configuration reference](/reference/configuration/)
* [CLI reference](/reference/cli/)
* [Troubleshooting](/how-to/troubleshooting/)

## Network boundary

The daemon does not provide TLS. Keep the default local address when possible.

If another device must reach the Console, use a trusted private network or place the daemon behind a TLS reverse proxy that you operate. Do not expose the Console directly to the public internet.

Every management call requires the admin session. Config writes, revision restore, and sign out also require a valid CSRF token.
