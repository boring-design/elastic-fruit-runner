---
title: What is Elastic Fruit Runner?
description: Why Elastic Fruit Runner uses one daemon to provide short lived GitHub Actions runners.
---

Elastic Fruit Runner is a daemon that turns one Apple Silicon Mac into a small GitHub Actions runner host.

It can provide three common environments:

| Environment | Backend |
|---|---|
| macOS arm64 | Tart virtual machine |
| Linux arm64 | Docker container |
| Linux amd64 | Docker container with host emulation |

## Why one host

Teams often need both macOS and Linux jobs, but a small team may not want a separate host or a Kubernetes cluster for each environment.

Apple Silicon can run macOS virtual machines and Linux containers on the same machine. Elastic Fruit Runner uses that ability while keeping the control plane in one local daemon.

The daemon can manage runner sets for GitHub organizations and repositories. Each runner set has its own backend, image, labels, and capacity.

## Why short lived runners

A permanent self hosted runner keeps state from earlier jobs and needs manual care when it stops working.

Elastic Fruit Runner creates a Just in Time runner when GitHub assigns work. The runner accepts one job, then its container or virtual machine is removed. This reduces shared state between jobs and allows an idle host to scale to zero runners.

[Runner lifecycle](/explanation/runner-lifecycle/) explains this flow.

## Why no Kubernetes

Kubernetes is useful when many machines and services need one shared control plane. It also adds a cluster that must be installed, secured, upgraded, and monitored.

Elastic Fruit Runner is built for one host. A local daemon is a smaller operating model for that scope. It uses the GitHub Runner Scale Set service directly and manages Docker and Tart resources on the host.

## Local operations

The embedded [Console](/reference/console/) shows runner capacity, jobs, host history, and config state for the same daemon. It is an operations view, not a central service for many hosts.

The Console follows a local, single admin security model. [Console design](/explanation/console-design/) describes the reasons and limits.

## Current status

Elastic Fruit Runner is a proof of concept. The main runner and Console flows work, but the project is not described as production ready.

It fits people who can operate and secure one Apple Silicon host and who accept the current proof of concept limits. Review the [Getting Started tutorial](/tutorials/getting-started/) and current repository Roadmap before using it for important workloads.
