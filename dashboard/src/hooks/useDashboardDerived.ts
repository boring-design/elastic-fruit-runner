import { useDashboardStore } from '../store/useDashboardStore'
import { elapsed } from '../utils'
import { REFRESH_INTERVAL_MS } from '../config'
import type { PetMood } from '../components/petMood'
import { deriveDashboardStatus } from '../derived/status'

export function useDashboardDerived() {
  const { daemonStatus, runnerSets, recentJobs, machineVitals, now, lastSyncedAt } = useDashboardStore()

  const uptime = daemonStatus ? elapsed(daemonStatus.startedAt, now) : 0

  const allRunners = runnerSets.flatMap(rs => rs.runners)
  const totalMax = runnerSets.reduce((s, rs) => s + rs.maxRunners, 0)
  const totalActive = allRunners.length
  const preparing = allRunners.filter(r => r.state === 'preparing').length
  const idle = allRunners.filter(r => r.state === 'idle').length
  const busy = allRunners.filter(r => r.state === 'busy').length
  const utilPct = totalMax > 0 ? Math.round((totalActive / totalMax) * 100) : 0

  const completedJobs = recentJobs.filter(j => j.result !== 'running')
  const successCount = completedJobs.filter(j => j.result === 'success').length
  const failureCount = completedJobs.filter(j => j.result === 'failure').length
  const canceledCount = completedJobs.filter(j => j.result === 'canceled').length

  const mood: PetMood =
    preparing > 0 ? 'alert' :
    busy > 0      ? 'busy'  :
    idle > 0      ? 'idle'  :
    'sleeping'

  const status = deriveDashboardStatus(runnerSets)

  const refreshIntervalSeconds = Math.max(1, Math.round(REFRESH_INTERVAL_MS / 1000))
  let secondsUntilRefresh = refreshIntervalSeconds
  if (lastSyncedAt) {
    const sinceLastSync = elapsed(lastSyncedAt, now)
    const remaining = refreshIntervalSeconds - sinceLastSync
    secondsUntilRefresh = remaining < 0
      ? 0
      : Math.min(remaining, refreshIntervalSeconds)
  }

  return {
    daemonStatus,
    runnerSets,
    recentJobs,
    machineVitals,
    now,
    uptime,
    totalMax,
    totalActive,
    preparing,
    idle,
    busy,
    utilPct,
    successCount,
    failureCount,
    canceledCount,
    mood,
    status,
    refreshIntervalSeconds,
    secondsUntilRefresh,
  }
}
