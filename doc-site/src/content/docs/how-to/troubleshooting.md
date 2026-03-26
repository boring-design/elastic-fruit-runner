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
