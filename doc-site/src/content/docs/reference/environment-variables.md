---
title: Environment Variables Reference
description: Environment variables supported by elastic-fruit-runner.
---

elastic-fruit-runner is primarily configured through its YAML config file. The following environment variables are recognised at startup:

| Variable | Config file equivalent | Description |
|----------|----------------------|-------------|
| `LOG_LEVEL` | `log_level` | Log level: `debug`, `info`, `warn`, `error`. Overrides the value in the config file |
| `EFR_TART_PRESERVE_FAILED_VMS` | _(none)_ | Debug-only. When set to `1`/`true`/`yes`/`on`, the Tart backend skips `tart stop`/`tart delete` for VMs that failed during `Run` (e.g. SSH readiness timeout). Successful VMs are still cleaned up normally. Useful for diagnosing macOS launchd / Tart bridge networking issues — manual cleanup with `tart stop <vm> && tart delete <vm>` is required afterwards. |

## Example

```sh
LOG_LEVEL=debug elastic-fruit-runner
```

```sh
# Preserve failed Tart VMs for offline inspection
EFR_TART_PRESERVE_FAILED_VMS=true elastic-fruit-runner
```

All other settings (orgs, repos, auth, runner sets, idle timeout) must be configured in the YAML config file. See the [configuration reference](/reference/configuration/) for details.
