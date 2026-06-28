---
title: Troubleshooting
description: Common problems and solutions when running elastic-fruit-runner.
---

## Inspect service status and logs on macOS

Use these checks when elastic-fruit-runner is installed through Homebrew and
running as a launchd service.

### Service status

Check whether Homebrew thinks the service is started:

```sh
brew services list | grep elastic-fruit-runner
```

Check the actual launchd job:

```sh
launchctl list | grep elastic-fruit-runner
```

For full launchd details, including the process ID, working directory, log
paths, and environment:

```sh
launchctl print "gui/$(id -u)/homebrew.mxcl.elastic-fruit-runner"
```

Check the running process directly:

```sh
ps aux | grep elastic-fruit-runner | grep -v grep
```

### Logs

The Homebrew service writes stdout and stderr to:

```text
/opt/homebrew/var/log/elastic-fruit-runner.log
```

Read recent logs:

```sh
tail -n 200 /opt/homebrew/var/log/elastic-fruit-runner.log
```

Follow logs while triggering a workflow:

```sh
tail -f /opt/homebrew/var/log/elastic-fruit-runner.log
```

Useful filters:

```sh
grep '"runnerSet":"wonder-mesh-linux-arm64"' /opt/homebrew/var/log/elastic-fruit-runner.log | tail -n 80
grep '"msg":"scale set ready"' /opt/homebrew/var/log/elastic-fruit-runner.log | tail
grep '"msg":"listening for jobs"' /opt/homebrew/var/log/elastic-fruit-runner.log | tail
grep '"msg":"start runner failed"' /opt/homebrew/var/log/elastic-fruit-runner.log | tail
```

### Config file

The default macOS config path is:

```text
~/.elastic-fruit-runner/config.yaml
```

Other search paths are:

```text
/opt/homebrew/var/elastic-fruit-runner/config.yaml
/usr/local/var/elastic-fruit-runner/config.yaml
/etc/elastic-fruit-runner/config.yaml
```

After changing config, restart the service:

```sh
brew services restart elastic-fruit-runner
```

Then confirm the new config took effect by checking for runner set startup
messages:

```sh
grep '"msg":"authenticating with GitHub App"' /opt/homebrew/var/log/elastic-fruit-runner.log | tail
grep '"msg":"scale set ready"' /opt/homebrew/var/log/elastic-fruit-runner.log | tail
```

### Docker backend checks

If the scale set is ready but a runner does not start, verify Docker under the
same PATH used by launchd:

```sh
env PATH=/opt/homebrew/bin:/opt/homebrew/sbin:/usr/bin:/bin:/usr/sbin:/sbin docker version
```

Test pulling the runner image:

```sh
env PATH=/opt/homebrew/bin:/opt/homebrew/sbin:/usr/bin:/bin:/usr/sbin:/sbin docker pull --platform linux/arm64 ghcr.io/quipper/actions-runner:2.332.0
```

If Docker reports `docker-credential-osxkeychain` is missing, check where the
credential helper is installed:

```sh
ls -l /opt/homebrew/bin/docker-credential-osxkeychain
ls -l /usr/local/bin/docker-credential-osxkeychain
```

The service PATH includes `/opt/homebrew/bin`, so the helper must be reachable
from that PATH when the daemon starts Docker.

## Jobs stuck in "queued" after making a repository public

**Symptom**: Workflows that previously ran fine on self-hosted runners stop being picked up after converting a repository from private to public. Jobs stay in `queued` state indefinitely. No errors appear in the controller logs.

**Cause**: The organization's runner group has `allows_public_repositories` set to `false` by default. When the repository was private, runners worked normally. After making it public, the runner group silently refuses to route jobs to the runners.

**Fix**: Enable public repository access on the runner group.

Via the GitHub UI:

1. Go to **Organization Settings > Actions > Runner groups**
2. Select the runner group (e.g. **Default**)
3. Check **Allow public repositories**

Via the GitHub API:

```sh
gh api -X PATCH orgs/YOUR-ORG/actions/runner-groups/1 \
  --input - <<< '{"allows_public_repositories": true}'
```

:::caution
This is a silent failure — there are no errors in the controller logs and no indication in the GitHub Actions UI beyond the job staying queued.
:::
