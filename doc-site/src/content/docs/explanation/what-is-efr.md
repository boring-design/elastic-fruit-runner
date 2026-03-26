---
title: What is elastic-fruit-runner?
description: Turn your Apple Silicon Mac into a multi-platform GitHub Actions runner farm.
---

**elastic-fruit-runner** is a daemon that turns a single Apple Silicon Mac into a multi-platform GitHub Actions runner farm. One machine simultaneously provides macOS arm64, Linux arm64, and Linux amd64 runners — automatically scaling up when jobs arrive and destroying them after completion.

## The problem

GitHub-hosted runners are expensive. macOS runners cost 10x the per-minute rate of Linux runners, and even Linux minutes add up fast for active teams. Many developers and small teams have powerful Apple Silicon machines sitting idle most of the day — a MacBook Pro on a desk, a Mac mini in a closet — but there's been no easy way to turn that spare compute into CI capacity.

Existing solutions like [actions-runner-controller (ARC)](https://github.com/actions/actions-runner-controller) work well for Kubernetes-based Linux runners, but they don't support macOS VMs on Apple Silicon hardware. And none of them let you run macOS, Linux arm64, and Linux amd64 workloads from a single machine.

## The solution

elastic-fruit-runner fills this gap. Install it on any Apple Silicon Mac and it becomes a multi-platform runner host:

| Runner type | Backend | How it works |
|---|---|---|
| **macOS arm64** | Tart | Ephemeral macOS VMs via Apple's Virtualization.framework |
| **Linux arm64** | Docker | Native Linux containers on ARM |
| **Linux amd64** | Docker | Linux containers via Rosetta 2 emulation |
| **Host** | Direct | Zero-overhead execution on the bare metal |

Under the hood, elastic-fruit-runner:

- **Uses the official GitHub Runner Scale Set API** to receive job signals in real time via long-polling, eliminating the need for webhook infrastructure
- **Creates ephemeral runners** with JIT (Just-In-Time) tokens — each runner accepts exactly one job, then self-terminates. No stale runner entries, no ghost runners blocking the queue
- **Scales to zero** when idle — no VMs or containers running when there are no jobs

## Who is it for?

- **Anyone with an Apple Silicon Mac** who wants to stop paying for GitHub Actions minutes
- Teams running CI/CD for macOS or iOS projects who are tired of expensive GitHub-hosted macOS runners
- Developers who need both macOS and Linux runners but don't want to manage Kubernetes or separate infrastructure
- People whose Mac mini is sitting idle running nothing but openclaw and deserve a better purpose in life

## Current status

elastic-fruit-runner is a proof of concept. The core flow (job detection, VM provisioning, runner execution, cleanup) works reliably. It is not yet production-hardened — see the [roadmap](https://github.com/boring-design/elastic-fruit-runner) for planned improvements.
