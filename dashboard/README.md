# elastic-fruit-runner · Dashboard

A terminal-style monitoring dashboard for [elastic-fruit-runner](../) — a GitHub Actions Runner Scale Set controller for Apple Silicon and Linux.

## Overview

The dashboard provides a real-time view of runner state, system health, and job history. It is designed for engineers who want dense, actionable information at a glance, with a dark monospace aesthetic inspired by terminal UIs.

### Features

- **Runner Brain** — a pixel-art mascot that reflects the runner's current mood (`STANDING BY`, `PROCESSING`, `SPINNING UP`, `RESTING`) with animated expressions
- **Runner Capacity** — overall and per-set utilization bars, average job duration, and success rate
- **System Vitals** — live-fluctuating CPU, memory, disk, and temperature readings
- **Connection Status** — GitHub org/repo target, auth mode, and last job timestamp
- **Recent Jobs** — scrollable history of completed and running jobs with result and duration
- **Responsive** — adapts from desktop (3-column) to tablet (2-column) to mobile (single-column)

## Tech Stack

| Layer | Choice |
|-------|--------|
| Framework | React 19 + TypeScript |
| Build | Vite |
| Styles | Tailwind CSS v4 + inline styles |
| Font | Geist Mono Variable |
| Data | Mock (ready to wire to a real API) |

## Development

```bash
# from the dashboard/ directory
npm install
npm run dev        # http://localhost:5173
npm run build      # output in dist/
npm run preview    # preview production build
```

## Wiring to Real Data

All mock data lives in `src/mock.ts`. The type contracts are in `src/types.ts`.

When the daemon exposes an HTTP API, replace the imports in `src/App.tsx`:

```ts
// Before (mock)
import { daemonStatus, runnerSets, recentJobs } from './mock'

// After (real API)
const { daemonStatus, runnerSets, recentJobs } = await fetch('/api/status').then(r => r.json())
```

The data shape expected by the dashboard:

```ts
DaemonStatus   // version, commitSha, startedAt, githubConnected, idleTimeout
RunnerSet[]    // name, backend, image, labels, maxRunners, scope, runners[]
JobRecord[]    // id, runnerName, runnerSetName, result, startedAt, completedAt
```

## Project Structure

```
src/
├── App.tsx                  # main layout and all page-level components
├── types.ts                 # shared TypeScript interfaces
├── mock.ts                  # mock data (replace with real API calls)
├── index.css                # global styles, CSS variables, responsive grid
├── main.tsx                 # React entry point
└── components/
    ├── PixelPet.tsx         # animated pixel-art mascot (canvas-based)
    └── SystemVitals.tsx     # CPU / memory / disk / temperature bars
```
