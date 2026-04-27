---
title: Environment Variables Reference
description: Environment variables supported by elastic-fruit-runner.
---

elastic-fruit-runner is primarily configured through its YAML config file. Only one environment variable is supported:

| Variable | Config file equivalent | Description |
|----------|----------------------|-------------|
| `LOG_LEVEL` | `log_level` | Log level: `debug`, `info`, `warn`, `error`. Overrides the value in config file |

## Example

```sh
LOG_LEVEL=debug elastic-fruit-runner
```

All other settings (orgs, repos, auth, runner sets, idle timeout) must be configured in the YAML config file. See the [configuration reference](/reference/configuration/) for details.
