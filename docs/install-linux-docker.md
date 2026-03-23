# Linux Deployment (Docker)

elastic-fruit-runner on Linux uses Docker-in-Docker: the daemon runs inside a container and spawns ephemeral runner containers on the host's Docker engine via a mounted socket.

## Quick start

```sh
docker run -d \
  --name elastic-fruit-runner \
  --restart unless-stopped \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -e GITHUB_TOKEN=ghp_xxx \
  -e GITHUB_CONFIG_URL=https://github.com/your-org \
  ghcr.io/boring-design/elastic-fruit-runner:latest
```

## Docker Compose

### With env vars (simple)

```yaml
# docker-compose.yaml
services:
  elastic-fruit-runner:
    image: ghcr.io/boring-design/elastic-fruit-runner:latest
    restart: unless-stopped
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
    environment:
      GITHUB_TOKEN: ghp_xxx
      GITHUB_CONFIG_URL: https://github.com/your-org
```

```sh
docker compose up -d
```

### With config file (full control)

Create a `config.yaml` (see [config.example.yaml](../config.example.yaml)):

```yaml
github:
  url: https://github.com/your-org
  token: ghp_xxx

runner_sets:
  - name: efr-linux-arm64
    backend: docker
    image: ghcr.io/actions-runner-controller/actions-runner-controller/actions-runner-dind:latest
    labels: [self-hosted, Linux, ARM64]
    max_runners: 4
    platform: linux/arm64
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

### With GitHub App auth

```yaml
# docker-compose.yaml
services:
  elastic-fruit-runner:
    image: ghcr.io/boring-design/elastic-fruit-runner:latest
    restart: unless-stopped
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
      - ./private-key.pem:/etc/efr/private-key.pem:ro
    environment:
      GITHUB_APP_CLIENT_ID: Iv1.xxxxxxxxxxxxxxxx
      GITHUB_APP_INSTALLATION_ID: "12345678"
      GITHUB_APP_PRIVATE_KEY_PATH: /etc/efr/private-key.pem
      GITHUB_CONFIG_URL: https://github.com/your-org
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

The daemon container mounts the host's Docker socket (`/var/run/docker.sock`). When a job arrives, it creates a sibling container (not nested) on the host engine using the `docker` CLI. Each runner container is destroyed after the job completes.

## Viewing logs

```sh
# Follow daemon logs
docker logs -f elastic-fruit-runner

# With docker compose
docker compose logs -f elastic-fruit-runner

# Filter errors with jq (logs are JSON)
docker logs elastic-fruit-runner 2>&1 | jq 'select(.level == "ERROR")'
```

## Updating

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
  --env-file /etc/elastic-fruit-runner/env \
  ghcr.io/boring-design/elastic-fruit-runner:latest
ExecStop=/usr/bin/docker stop elastic-fruit-runner

[Install]
WantedBy=multi-user.target
```

```sh
sudo systemctl enable --now elastic-fruit-runner
sudo journalctl -fu elastic-fruit-runner
```
