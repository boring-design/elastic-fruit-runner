# Elastic Fruit Runner Console

The dashboard is the embedded operations console for one Elastic Fruit Runner daemon.

It provides Overview, Jobs, Runner Sets, Config, and System pages. Data comes from the Connect RPC service in the daemon.

## Development

```sh
pnpm install
pnpm run dev
pnpm run build
pnpm run lint
```

Set `VITE_API_BASE` only when the dashboard development server and daemon use different origins.

The production build is written to `dist` and embedded in the Go binary.

## Security

The console uses a one time setup code, one admin password, an HttpOnly session cookie, and CSRF tokens for changes.

Secret values are masked by the API. A masked PAT in the editor keeps the current disk value when saved.
