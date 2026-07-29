import { useEffect } from 'react'
import useSWR from 'swr'
import { useDashboardStore } from '../store/useDashboardStore'
import { fetchDaemonStatus, fetchRunnerSets, fetchRecentJobs, fetchMachineVitals } from '../api/fetchers'
import { REFRESH_INTERVAL_MS } from '../config'

export function useDashboardSync() {
  const {
    setDaemonStatus,
    setRunnerSets,
    setRecentJobs,
    setMachineVitals,
    markSynced,
    tick,
  } = useDashboardStore()

  const status = useSWR('daemonStatus', fetchDaemonStatus, {
    refreshInterval: REFRESH_INTERVAL_MS,
    onSuccess: (data) => {
      setDaemonStatus(data)
      markSynced()
    },
  })

  const sets = useSWR('runnerSets', fetchRunnerSets, {
    refreshInterval: REFRESH_INTERVAL_MS,
    onSuccess: (data) => {
      setRunnerSets(data)
      markSynced()
    },
  })

  const jobs = useSWR('recentJobs', fetchRecentJobs, {
    refreshInterval: REFRESH_INTERVAL_MS,
    onSuccess: (data) => {
      setRecentJobs(data)
      markSynced()
    },
  })

  const vitals = useSWR('machineVitals', fetchMachineVitals, {
    refreshInterval: REFRESH_INTERVAL_MS,
    onSuccess: (data) => {
      setMachineVitals(data)
      markSynced()
    },
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
