---
title: Troubleshooting
description: Common problems and solutions when running elastic-fruit-runner.
---

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

## Tart VMs fail to start from `brew services` ("no route to host")

**Symptom**: Running `brew services start elastic-fruit-runner` on macOS 15 (Sequoia) or later, every macOS runner gets stuck preparing. Logs show repeated `waiting for SSH` lines followed by:

```text
start runner failed err="... SSH not reachable on <vm> (192.168.64.X:22) after 2m0s: last error:
ssh readiness probe ... ssh: connect to host 192.168.64.X port 22: No route to host (route: iface=bridge101 gateway=192.168.64.1)"
```

Crucially, running `sshpass ssh admin@192.168.64.X true` from an interactive Terminal at the same time **succeeds**.

**Cause**: macOS 15 introduced [Local Network Privacy](https://developer.apple.com/documentation/technotes/tn3179-understanding-local-network-privacy). Daemons started under launchd as **LaunchAgents** (which is what `brew services start` produces when run without `sudo`) need a Mach-O `LC_UUID` load command for the kernel to identify the binary and route packets to private subnets such as `192.168.64.0/24` (the Tart bridge).

Older Go toolchains produced binaries without `LC_UUID`, which is why `ssh` (Apple-signed system binary) worked while the Go binary failed with `connect: no route to host`. See [golang/go#68678](https://github.com/golang/go/issues/68678) and [cirruslabs/orchard#221](https://github.com/cirruslabs/orchard/issues/221) for the upstream context.

**Fix**:

1. **Upgrade**: builds since the fix carry the `-B gobuildid` ldflag and embed `LC_UUID`.

   ```sh
   brew update && brew upgrade elastic-fruit-runner
   brew services restart elastic-fruit-runner
   ```

2. **Verify the binary has `LC_UUID`**:

   ```sh
   otool -l "$(brew --prefix)/opt/elastic-fruit-runner/bin/elastic-fruit-runner" | grep -A1 LC_UUID
   ```

   You should see a `cmd LC_UUID` entry. If it is missing, the binary will not work as a LaunchAgent on macOS 15+.

3. **Workaround — run as a system LaunchDaemon**: LaunchDaemons (`/Library/LaunchDaemons/`, loaded as root) bypass Local Network Privacy entirely. Stop the per-user agent and start the service as root:

   ```sh
   brew services stop elastic-fruit-runner
   sudo brew services start elastic-fruit-runner
   ```

   :::note
   `sudo brew services start` installs a system-wide LaunchDaemon under `/Library/LaunchDaemons/` and runs the binary as root. This makes the network restriction go away but means the runner has root privileges. Pick whichever trade-off fits your security posture.
   :::

4. **Diagnose with the preserve-failed-VMs debug knob**: if you still hit the failure mode after the upgrade, set `EFR_TART_PRESERVE_FAILED_VMS=true` in the service environment and restart. Failed VMs will not be deleted, so you can `tart ip <vm>` and `tart ssh <vm>` to inspect them, and `route -n get <vm-ip>` from inside the daemon's environment to confirm what interface it picks.

   See [environment variables reference](/reference/environment-variables/).
