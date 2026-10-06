---
title: How to run in cloud mode
description: Connect a host to Elastic Fruit Cloud so the cloud talks to GitHub and the host only runs jobs.
---

## What cloud mode is

In standalone mode the daemon holds GitHub credentials, registers runner scale sets, and decides when to start a runner.

In cloud mode an Elastic Fruit Cloud server does all of that. The daemon on your host becomes an agent. It opens one connection to the cloud, receives commands such as start a runner or remove a runner, and reports back what happened. The host needs no GitHub credentials.

Everything about a job still stays on the host. Job history, runner logs, and resource samples are written to the local SQLite database, and the local Console keeps working.

## Enroll the host

Create an enrollment token in the cloud console, then run on the host:

```sh
elastic-fruit-runner enroll --server https://cloud.example.com --token <token> --max-runners 2
```

The command:

1. Checks which runner backends work on this host, docker and tart.
2. Registers the host with the cloud using the one time token.
3. Writes the agent credential to `~/.elastic-fruit-runner/agent-credential`, readable only by the current user.
4. Writes or updates the `cloud` block in the config file and records a config revision.

If the config file does not exist, the command creates one with only the `cloud` block. If the config file still has `orgs` or `repos`, the command stops and asks you to remove them first, because cloud mode replaces them.

Use `--config PATH` to pick a different config file. Plain `http://` server URLs are accepted only for `localhost` and `127.0.0.1`.

The resulting config looks like this:

```yaml
cloud:
  server_url: https://cloud.example.com
  max_runners: 2
```

## Start the daemon

Start or restart the service the same way as in standalone mode:

```sh
elastic-fruit-runner
```

or with a service manager:

```sh
brew services restart elastic-fruit-runner
sudo systemctl restart elastic-fruit-runner
```

The daemon refuses to start in cloud mode when the credential file is missing. The error names the enroll command to run.

## What the agent does while running

* It keeps a command stream to the cloud open. When the stream drops, it reconnects with a growing delay between one and thirty seconds.
* It sends a heartbeat every ten seconds with host resource usage and the state of every runner.
* It reports each runner start, start failure, and cleanup back to the cloud.
* It uploads the resource samples captured while a job runs.

When the stream is down no new runners start. Runners that are already running keep running and finish their jobs. Stopping the daemon does not stop running runners either.

## The Console in cloud mode

The Console shows a banner with the cloud server URL. The setup wizard and the GitHub checks are hidden. The setup checklist has a **Cloud connected** step that follows the command stream. The Config editor and the restart button keep working.

## Leave cloud mode

1. Remove the host in the cloud console.
2. Delete the `cloud` block from the config file.
3. Delete the credential file:

```sh
rm ~/.elastic-fruit-runner/agent-credential
```

4. Add `orgs` or `repos` back to the config file for standalone mode, see [How to configure a GitHub App](/how-to/configure-github-app/).
5. Restart the daemon.
