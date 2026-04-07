---
title: What is elastic-fruit-runner?
description: Turn your Apple Silicon Mac into a multi-platform GitHub Actions runner farm.
---

**elastic-fruit-runner** is a daemon that turns a single Apple Silicon Mac into a multi-platform GitHub Actions runner farm. One machine simultaneously provides macOS arm64, Linux arm64, and Linux amd64 runners — automatically scaling up when jobs arrive and destroying them after completion.

## The problems

### 1. Too many machine types, never enough runners

GitHub Actions workflows target a wide variety of platforms — macOS arm64 for iOS builds, Linux arm64 for ARM deployments, Linux amd64 for the rest. Traditionally, each platform needed its own dedicated machine or VM. If you wanted full coverage, you were looking at multiple machines or expensive GitHub-hosted runner minutes.

Thanks to Apple Silicon, that constraint is gone. A single Mac can natively run macOS arm64, Linux arm64 (via Docker), and Linux amd64 (via Docker + Rosetta 2). One machine, three platforms — you just need software that knows how to orchestrate them.

### 2. Runner management is painful

The traditional self-hosted runner experience is full of friction:

- **Per-repo and per-org registration** — every repository or organization needs its own runner registration. Managing tokens and runner entries across dozens of repos is tedious and error-prone.
- **Always-on standby** — traditional runners must stay running 24/7, burning CPU and memory even when there are no jobs. You're paying the cost of idle infrastructure just to be ready.
- **Stale runners and ghost entries** — when a runner crashes or a machine reboots, you end up with orphaned runner entries in GitHub that block the job queue until you manually clean them up.

### 3. Existing solutions require Kubernetes

[actions-runner-controller (ARC)](https://github.com/actions/actions-runner-controller) is an excellent project that solved elastic runner scaling for Linux. But it requires Kubernetes. If you just have a Mac mini under your desk, the path to ARC looks like: install minikube (or kind, or k3s), then deploy ARC into the cluster, then manage Kubernetes on top of managing runners. Running a full Kubernetes stack on a Mac mini just to get elastic CI runners is absurd overkill.

## The solution

elastic-fruit-runner throws all of that away. Install it on any Apple Silicon Mac and it becomes a multi-platform runner farm:

| Runner type | Backend | How it works |
|---|---|---|
| **macOS arm64** | Tart | Ephemeral macOS VMs via Apple's Virtualization.framework |
| **Linux arm64** | Docker | Native Linux containers on ARM |
| **Linux amd64** | Docker | Linux containers via Rosetta 2 emulation |

Powered by the official [GitHub Runner Scale Set API](https://github.com/actions/scaleset), elastic-fruit-runner fundamentally changes how self-hosted runners work:

- **One daemon manages everything** — no per-repo registration, no per-org token juggling. Configure your orgs and repos in a single YAML file, and the daemon handles all scale set registration and lifecycle management.
- **Scale to zero** — when there are no jobs, there are no runners. No idle VMs, no idle containers, no wasted resources. The daemon itself is a lightweight long-polling process that consumes almost nothing.
- **Scale up on demand** — when a job arrives, the daemon instantly provisions an ephemeral runner (VM or container), runs the job, and destroys it. Each runner accepts exactly one job via JIT (Just-In-Time) tokens — no stale entries, no ghost runners.
- **No Kubernetes required** — just `brew install` and a config file. That's it. No clusters, no operators, no YAML manifests.

## Who is it for?

- **Anyone with an Apple Silicon Mac** who wants to stop paying for GitHub Actions minutes
- Teams running CI/CD for macOS or iOS projects who are tired of expensive GitHub-hosted macOS runners
- Developers who need both macOS and Linux runners but don't want to manage Kubernetes or separate infrastructure
- People whose Mac mini is sitting idle running nothing but openclaw and deserve a better purpose in life

## Current status

elastic-fruit-runner is a proof of concept. The core flow (job detection, VM provisioning, runner execution, cleanup) works reliably. It is not yet production-hardened — see the [roadmap](https://github.com/boring-design/elastic-fruit-runner) for planned improvements.
