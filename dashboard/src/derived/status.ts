import type { RunnerSet } from '../types'

export type DashboardStatusKind =
  | 'loading'
  | 'disconnected'
  | 'connecting'
  | 'scaling-up'
  | 'processing'
  | 'listening'
  | 'idle'

export interface DashboardStatus {
  kind: DashboardStatusKind
  label: string
  detail: string
  tone: 'neutral' | 'active' | 'warning' | 'error'
}

const STATUS_DETAIL: Record<DashboardStatusKind, string> = {
  loading: 'AWAITING DATA',
  disconnected: 'CONTROLLER OFFLINE',
  connecting: 'GITHUB SCALE SET API',
  'scaling-up': 'PREPARING NEW RUNNERS',
  processing: 'RUNNERS EXECUTING JOBS',
  listening: 'GITHUB SCALE SET API',
  idle: 'GITHUB SCALE SET API',
}

const STATUS_LABEL: Record<DashboardStatusKind, string> = {
  loading: 'INITIALIZING',
  disconnected: 'DISCONNECTED',
  connecting: 'CONNECTING',
  'scaling-up': 'SCALING UP',
  processing: 'PROCESSING',
  listening: 'LISTENING FOR JOBS',
  idle: 'IDLE',
}

const STATUS_TONE: Record<DashboardStatusKind, DashboardStatus['tone']> = {
  loading: 'neutral',
  disconnected: 'error',
  connecting: 'warning',
  'scaling-up': 'active',
  processing: 'active',
  listening: 'neutral',
  idle: 'neutral',
}

function buildStatus(kind: DashboardStatusKind): DashboardStatus {
  return {
    kind,
    label: STATUS_LABEL[kind],
    detail: STATUS_DETAIL[kind],
    tone: STATUS_TONE[kind],
  }
}

// deriveDashboardStatus collapses the controller's runner-set view into a
// single high-level operational status the dashboard can render.
//
// Rules (in priority order):
//   1. No runner-set data yet -> loading.
//   2. Every set reports disconnected -> disconnected.
//   3. At least one set is disconnected (mixed) -> connecting.
//   4. Any runner is preparing -> scaling-up.
//   5. Any runner is busy -> processing.
//   6. Any runner is idle -> listening.
//   7. No runners at all -> idle.
export function deriveDashboardStatus(runnerSets: RunnerSet[]): DashboardStatus {
  if (runnerSets.length === 0) {
    return buildStatus('loading')
  }

  const connectedCount = runnerSets.filter(rs => rs.connected).length
  if (connectedCount === 0) {
    return buildStatus('disconnected')
  }
  if (connectedCount < runnerSets.length) {
    return buildStatus('connecting')
  }

  const allRunners = runnerSets.flatMap(rs => rs.runners)
  const hasPreparing = allRunners.some(r => r.state === 'preparing')
  if (hasPreparing) {
    return buildStatus('scaling-up')
  }

  const hasBusy = allRunners.some(r => r.state === 'busy')
  if (hasBusy) {
    return buildStatus('processing')
  }

  const hasIdle = allRunners.some(r => r.state === 'idle')
  if (hasIdle) {
    return buildStatus('listening')
  }

  return buildStatus('idle')
}
