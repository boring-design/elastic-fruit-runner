---
title: How to install gVisor on Ubuntu
description: Install the runsc runtime so Docker runner containers run inside a gVisor sandbox.
---

[gVisor](https://gvisor.dev) is a sandbox for containers. Its `runsc` runtime puts a user space kernel between the container and the host, so a job that breaks out of the runner process still cannot reach the host kernel. Install it on a Linux host when the host runs jobs from people you do not fully trust, for example as part of a shared pool in cloud mode.

Follow the official guide at [gvisor.dev/docs/user_guide/install](https://gvisor.dev/docs/user_guide/install/) if these steps go out of date.

## Install runsc from the apt repository

```sh
sudo apt-get update && sudo apt-get install -y \
  apt-transport-https \
  ca-certificates \
  curl \
  gnupg

curl -fsSL https://gvisor.dev/archive.key | sudo gpg --dearmor -o /usr/share/keyrings/gvisor-archive-keyring.gpg
echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/gvisor-archive-keyring.gpg] https://storage.googleapis.com/gvisor/releases release main" | sudo tee /etc/apt/sources.list.d/gvisor.list > /dev/null

sudo apt-get update && sudo apt-get install -y runsc
```

gVisor needs Linux 5.6 or newer on x86_64 or ARM64.

## Register runsc with Docker

```sh
sudo runsc install
sudo systemctl reload docker
```

`runsc install` adds a `runsc` entry to `/etc/docker/daemon.json`. The reload makes Docker pick it up.

## Verify

```sh
docker run --rm --runtime=runsc hello-world
docker info --format '{{json .Runtimes}}'
```

The first command prints the hello world message. The second lists `runsc` among the runtimes.

## Restart the daemon

elastic-fruit-runner probes the Docker runtimes once when it starts. This matters in cloud mode, where the agent reports the isolation level to the cloud. Restart it after installing gVisor so the report includes `runsc`:

```sh
sudo systemctl restart elastic-fruit-runner
```

If you run it with Docker Compose, restart it with `docker compose restart`. In cloud mode the agent then reports the isolation level `sandboxed_container`.

In standalone mode no restart is needed. The runtime check runs before each container start. Set `runtime: runsc` on a Docker runner set, see [configuration reference](/reference/configuration/).

## Docker in Docker does not work under the sandbox

With `runtime: runsc` the runner container starts without `--privileged`, and the strict sandbox does not let a Docker daemon run inside it. Images such as `actions-runner-dind` fail to start. Use a plain runner image such as `ghcr.io/actions/actions-runner`, and expect that jobs cannot call `docker` inside a sandboxed runner.

## No KVM needed

gVisor uses the `systrap` platform by default. It does not require KVM, so it works inside a cloud virtual machine where nested virtualization is off. gVisor notes that within a virtual machine, systrap will often yield better performance than KVM.
