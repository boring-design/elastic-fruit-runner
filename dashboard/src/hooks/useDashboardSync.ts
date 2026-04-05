import { useEffect } from 'react'
import useSWR from 'swr'
import { useDashboardStore } from '../store/useDashboardStore'
import { fetchDaemonStatus, fetchRunnerSets, fetchRecentJobs, fetchMachineVitals } from '../api/fetchers'

const REFRESH_INTERVAL = 5000

export function useDashboardSync() {
  const { setDaemonStatus, setRunnerSets, setRecentJobs, setMachineVitals, tick } = useDashboardStore()

  const status = useSWR('daemonStatus', fetchDaemonStatus, {
    refreshInterval: REFRESH_INTERVAL,
    onSuccess: setDaemonStatus,
  })

  const sets = useSWR('runnerSets', fetchRunnerSets, {
    refreshInterval: REFRESH_INTERVAL,
    onSuccess: setRunnerSets,
  })

  const jobs = useSWR('recentJobs', fetchRecentJobs, {
    refreshInterval: REFRESH_INTERVAL,
    onSuccess: setRecentJobs,
  })

  const vitals = useSWR('machineVitals', fetchMachineVitals, {
    refreshInterval: REFRESH_INTERVAL,
    onSuccess: setMachineVitals,
  })

  useEffect(() => {
    const id = setInterval(tick, 1000)
    return () => clearInterval(id)
  }, [tick])

  return {
    isLoading: status.isLoading || sets.isLoading || jobs.isLoading || vitals.isLoading,
    error: status.error ?? sets.error ?? jobs.error ?? vitals.error ?? null,
  }
}
