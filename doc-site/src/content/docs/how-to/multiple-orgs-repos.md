---
title: How to manage multiple orgs and repos
description: Configure elastic-fruit-runner to serve multiple GitHub organizations and repositories simultaneously.
---

A single elastic-fruit-runner daemon can manage runners for multiple GitHub organizations and individual repositories at the same time.

## Configure multiple organizations

Add multiple entries under `orgs`, each with its own auth and runner sets:

```yaml
orgs:
  - org: org-one
    auth:
      github_app:
        client_id: Iv1.aaa
        installation_id: 11111111
        private_key_path: /path/to/org-one-key.pem
    runner_group: Default
    runner_sets:
      - name: org-one-macos
        backend: tart
        image: ghcr.io/cirruslabs/macos-tahoe-xcode:26.3
        labels: [self-hosted, macos, arm64]
        max_runners: 2

  - org: org-two
    auth:
      pat_token: ghp_xxx
    runner_group: Default
    runner_sets:
      - name: org-two-linux
        backend: docker
        image: ghcr.io/actions-runner-controller/actions-runner-controller/actions-runner-dind:latest
        labels: [self-hosted, linux, arm64]
        max_runners: 4
        platform: linux/arm64
```

## Configure repository-level runners

Use `repos` for repository-scoped runners. Each entry uses `owner/repo` format:

```yaml
repos:
  - repo: your-org/your-repo
    auth:
      pat_token: ghp_xxx
    runner_sets:
      - name: repo-runner
        backend: docker
        image: ghcr.io/actions-runner-controller/actions-runner-controller/actions-runner-dind:latest
        labels: [self-hosted, linux, arm64]
        max_runners: 2
```

Repository-level runners do not use `runner_group` — they use the default group.

## Mix organizations and repositories

You can combine `orgs` and `repos` in a single config:

```yaml
orgs:
  - org: my-org
    auth:
      github_app:
        client_id: Iv1.xxx
        installation_id: 12345678
        private_key_path: /path/to/key.pem
    runner_group: Default
    runner_sets:
      - name: org-macos
        backend: tart
        image: ghcr.io/cirruslabs/macos-tahoe-xcode:26.3
        labels: [self-hosted, macos, arm64]
        max_runners: 2

repos:
  - repo: other-org/special-repo
    auth:
      pat_token: ghp_yyy
    runner_sets:
      - name: special-linux
        backend: docker
        image: ghcr.io/actions-runner-controller/actions-runner-controller/actions-runner-dind:latest
        labels: [self-hosted, linux, arm64]
        max_runners: 2

idle_timeout: 15m
```

Each org and repo gets its own controller goroutine, so they poll and scale independently.
