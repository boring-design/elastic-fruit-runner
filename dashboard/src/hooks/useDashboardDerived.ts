import { useDashboardStore } from '../store/useDashboardStore'
import { elapsed } from '../utils'
import type { PetMood } from '../components/petMood'

export function useDashboardDerived() {
  const { daemonStatus, runnerSets, recentJobs, machineVitals, now } = useDashboardStore()

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
  // Jobs whose result string the daemon could not classify. They are
  // counted as completed (so the COMPLETED total moves forward) but
  // deliberately excluded from the success/failure split — see issue #68
  // where "unknown" results were silently treated as failures.
  const unknownCount = completedJobs.filter(j => j.result === 'unknown').length

  const mood: PetMood =
    preparing > 0 ? 'alert' :
    busy > 0      ? 'busy'  :
    idle > 0      ? 'idle'  :
    'sleeping'

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
    unknownCount,
    mood,
  }
}
