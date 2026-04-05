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
    mood,
  }
}
