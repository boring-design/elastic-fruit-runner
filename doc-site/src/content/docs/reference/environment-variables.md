---
title: Environment Variables Reference
description: Environment variables supported by elastic-fruit-runner.
---

Elastic Fruit Runner is configured through its YAML file. Only one environment variable is supported:

| Variable | Config file equivalent | Description |
|----------|----------------------|-------------|
| `LOG_LEVEL` | `log_level` | Log level: `debug`, `info`, `warn`, `error`. Overrides the value in config file |

## Example

```sh
LOG_LEVEL=debug elastic-fruit-runner
```

`LOG_LEVEL` has higher priority than the YAML `log_level` field.

All other settings must be configured in the YAML file. See [Configuration Reference](/reference/configuration/).
