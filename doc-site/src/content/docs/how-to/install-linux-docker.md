---
title: How to deploy on Linux with Docker
description: Deploy elastic-fruit-runner on Linux using Docker or Docker Compose.
---

On Linux, elastic-fruit-runner runs inside a container and spawns ephemeral runner containers on the host's Docker engine via a mounted socket.

## Docker Compose with config file

Create a `config.yaml` (see [configuration reference](/reference/configuration/)):

```yaml
orgs:
  - org: your-org
    auth:
      pat_token: ghp_xxx
    runner_group: Default
    runner_sets:
      # Linux arm64 runners
      - name: efr-linux-arm64
        backend: docker
        image: ghcr.io/actions-runner-controller/actions-runner-controller/actions-runner-dind:latest
        labels: [self-hosted, linux, arm64]
        max_runners: 4
        platform: linux/arm64

      # Linux amd64 runners (Rosetta 2 emulation on Apple Silicon)
      - name: efr-linux-amd64
        backend: docker
        image: ghcr.io/actions-runner-controller/actions-runner-controller/actions-runner-dind:latest
        labels: [self-hosted, linux, amd64]
        max_runners: 4
        platform: linux/amd64

idle_timeout: 15m
```

```yaml
# docker-compose.yaml
services:
  elastic-fruit-runner:
    image: ghcr.io/boring-design/elastic-fruit-runner:latest
    restart: unless-stopped
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
      - ./config.yaml:/etc/elastic-fruit-runner/config.yaml:ro
```

```sh
docker compose up -d
```

## Docker Compose with GitHub App auth

```yaml
# config.yaml
orgs:
  - org: your-org
    auth:
      github_app:
        client_id: Iv1.xxxxxxxxxxxxxxxx
        installation_id: 12345678
        private_key_path: /etc/efr/private-key.pem
    runner_group: Default
    runner_sets:
      # Linux arm64 runners
      - name: efr-linux-arm64
        backend: docker
        image: ghcr.io/actions-runner-controller/actions-runner-controller/actions-runner-dind:latest
        labels: [self-hosted, linux, arm64]
        max_runners: 4
        platform: linux/arm64

      # Linux amd64 runners (Rosetta 2 emulation on Apple Silicon)
      - name: efr-linux-amd64
        backend: docker
        image: ghcr.io/actions-runner-controller/actions-runner-controller/actions-runner-dind:latest
        labels: [self-hosted, linux, amd64]
        max_runners: 4
        platform: linux/amd64

idle_timeout: 15m
```

```yaml
# docker-compose.yaml
services:
  elastic-fruit-runner:
    image: ghcr.io/boring-design/elastic-fruit-runner:latest
    restart: unless-stopped
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
      - ./config.yaml:/etc/elastic-fruit-runner/config.yaml:ro
      - ./private-key.pem:/etc/efr/private-key.pem:ro
```

## How it works

```
Host Docker Engine
├── elastic-fruit-runner container (long-running daemon)
│   └── polls GitHub Scale Set API for jobs
├── efr-linux-arm64-xxx container (ephemeral, one per job)
├── efr-linux-amd64-xxx container (ephemeral, one per job)
└── ...
```

The daemon container mounts the host's Docker socket (`/var/run/docker.sock`). When a job arrives, it creates a sibling container (not nested) on the host engine. Each runner container is destroyed after the job completes.

## View logs

```sh
docker logs -f elastic-fruit-runner

# With docker compose
docker compose logs -f elastic-fruit-runner

# Filter errors with jq (logs are JSON)
docker logs elastic-fruit-runner 2>&1 | jq 'select(.level == "ERROR")'
```

## Update

```sh
docker pull ghcr.io/boring-design/elastic-fruit-runner:latest
docker compose up -d
```

## systemd integration (optional)

If you prefer systemd to manage the container lifecycle:

```ini
# /etc/systemd/system/elastic-fruit-runner.service
[Unit]
Description=Elastic Fruit Runner
After=docker.service
Requires=docker.service

[Service]
Restart=always
ExecStartPre=-/usr/bin/docker rm -f elastic-fruit-runner
ExecStart=/usr/bin/docker run --rm --name elastic-fruit-runner \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v /etc/elastic-fruit-runner/config.yaml:/etc/elastic-fruit-runner/config.yaml:ro \
  ghcr.io/boring-design/elastic-fruit-runner:latest
ExecStop=/usr/bin/docker stop elastic-fruit-runner

[Install]
WantedBy=multi-user.target
```

```sh
sudo systemctl enable --now elastic-fruit-runner
sudo journalctl -fu elastic-fruit-runner
```
