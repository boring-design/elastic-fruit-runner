---
title: How to configure GitHub App authentication
description: Set up GitHub App auth for elastic-fruit-runner instead of a Personal Access Token.
---

GitHub App authentication is recommended for organization-wide deployments because it:

- **Doesn't expire** — unlike PATs, no manual rotation needed
- **Fine-grained permissions** — scoped exactly to what the runner daemon needs
- **Bound to the org** — not tied to a personal account, survives employee offboarding

## Step 1: Create a GitHub App

1. Go to your organization's settings: `https://github.com/organizations/YOUR-ORG/settings/apps`
2. Click **New GitHub App**
3. Set the following:
   - **GitHub App name**: `elastic-fruit-runner` (or any name)
   - **Homepage URL**: your org's URL
   - **Webhook**: uncheck **Active** (not needed)
4. Under **Permissions**, set:
   - **Organization permissions > Self-hosted runners**: **Read and write**
5. Click **Create GitHub App**

## Step 2: Generate a private key

1. On the App's settings page, scroll to **Private keys**
2. Click **Generate a private key**
3. Save the downloaded `.pem` file to a secure location (e.g., `/etc/elastic-fruit-runner/private-key.pem`)

## Step 3: Install the App

1. On the App's settings page, click **Install App** in the sidebar
2. Select your organization
3. Choose **All repositories** or select specific repositories
4. Click **Install**
5. Note the **Installation ID** from the URL: `https://github.com/organizations/YOUR-ORG/settings/installations/INSTALLATION_ID`

## Step 4: Note the Client ID

On the App's settings page, find the **Client ID** (starts with `Iv1.`).

## Step 5: Update configuration

Add the GitHub App credentials to your config file:

```yaml
orgs:
  - org: your-org
    auth:
      github_app:
        client_id: Iv1.xxxxxxxxxxxxxxxx
        installation_id: 12345678
        private_key_path: /path/to/private-key.pem
    runner_group: Default
    runner_sets:
      - name: efr-macos-arm64
        backend: tart
        image: ghcr.io/cirruslabs/macos-tahoe-xcode:26.3
        labels: [self-hosted, macos, arm64]
        max_runners: 2
      - name: efr-linux-arm64
        backend: docker
        image: ghcr.io/actions-runner-controller/actions-runner-controller/actions-runner-dind:latest
        labels: [self-hosted, linux, arm64]
        max_runners: 4
        platform: linux/arm64
      - name: efr-linux-amd64
        backend: docker
        image: ghcr.io/actions-runner-controller/actions-runner-controller/actions-runner-dind:latest
        labels: [self-hosted, linux, amd64]
        max_runners: 4
        platform: linux/amd64

idle_timeout: 15m
```

## Step 6: Restart the service

```sh
# macOS
brew services restart elastic-fruit-runner

# Linux (Docker)
docker compose restart elastic-fruit-runner
```

## Troubleshooting

### Permissions not saved

When creating the GitHub App in the browser, Organization permissions (especially **Self-hosted runners**) may appear as "No access" if the page didn't save correctly. Always verify at:

```
https://github.com/organizations/YOUR-ORG/settings/apps/APP-NAME/permissions
```

### Permission changes require re-acceptance

After updating permissions on an existing GitHub App, you must go to the installation page and explicitly **accept the new permissions**. The approval banner will list all new/changed permissions. Until you accept, the App still operates with the old permissions.

### Private key generation may need retry

The "Generate a private key" button on the GitHub App settings page occasionally fails silently. If you don't see a download prompt after clicking, scroll down and try again, or reload the page.
