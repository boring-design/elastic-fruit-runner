---
title: What is elastic-fruit-runner?
description: An overview of what elastic-fruit-runner is and why it exists.
---

**elastic-fruit-runner** is a daemon that manages elastic, ephemeral GitHub Actions self-hosted runners on Apple Silicon Macs. It automatically scales runners up when jobs arrive and destroys them after completion.

## The problem

GitHub-hosted runners for macOS are expensive and often queue for minutes during peak hours. Self-hosted runners avoid these costs but traditionally require manual setup, long-lived VMs, and careful lifecycle management.

Existing solutions like [actions-runner-controller (ARC)](https://github.com/actions/actions-runner-controller) work well for Kubernetes-based Linux runners, but they don't support macOS VMs on Apple Silicon hardware.

## The solution

elastic-fruit-runner fills this gap by:

- **Using the official GitHub Runner Scale Set API** to receive job signals in real time via long-polling, eliminating the need for webhook infrastructure
- **Creating ephemeral runners** with JIT (Just-In-Time) tokens — each runner accepts exactly one job, then self-terminates. No stale runner entries, no ghost runners blocking the queue
- **Supporting multiple backends** to match different workload needs:
  - **Tart** — ephemeral macOS VMs on Apple Silicon
  - **Docker** — Linux arm64 and amd64 containers (amd64 via Rosetta 2)
  - **Host** — direct execution with zero overhead

## Who is it for?

- Teams running CI/CD for macOS or iOS projects on Apple Silicon hardware
- Organizations that want elastic self-hosted runners without Kubernetes
- Developers who need both macOS and Linux runners from a single Apple Silicon machine

## Current status

elastic-fruit-runner is a proof of concept. The core flow (job detection, VM provisioning, runner execution, cleanup) works reliably. It is not yet production-hardened — see the [roadmap](https://github.com/boring-design/elastic-fruit-runner) for planned improvements.
