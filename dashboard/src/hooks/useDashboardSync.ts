import { useEffect } from 'react'
import useSWR from 'swr'
import {
  fetchConfigStatus,
  fetchDaemonStatus,
  fetchDashboardSummary,
  fetchMachineVitals,
  fetchRecentJobs,
  fetchRunnerSets,
  fetchSystemInfo,
} from '../api/fetchers'
import { useDashboardStore } from '../store/useDashboardStore'

const REFRESH_INTERVAL = 5000

export function useDashboardSync() {
  const store = useDashboardStore()
  const status = useSWR('daemonStatus', fetchDaemonStatus, {
    refreshInterval: REFRESH_INTERVAL,
    onSuccess: store.setDaemonStatus,
  })
  const summary = useSWR('dashboardSummary', fetchDashboardSummary, {
    refreshInterval: REFRESH_INTERVAL,
    onSuccess: store.setSummary,
  })
  const sets = useSWR('runnerSets', fetchRunnerSets, {
    refreshInterval: REFRESH_INTERVAL,
    onSuccess: store.setRunnerSets,
  })
  const jobs = useSWR('recentJobs', fetchRecentJobs, {
    refreshInterval: REFRESH_INTERVAL,
    onSuccess: store.setRecentJobs,
  })
  const vitals = useSWR('machineVitals', fetchMachineVitals, {
    refreshInterval: REFRESH_INTERVAL,
    onSuccess: store.setMachineVitals,
  })
  const config = useSWR('configStatus', fetchConfigStatus, {
    refreshInterval: REFRESH_INTERVAL,
    onSuccess: store.setConfigStatus,
  })
  const system = useSWR('systemInfo', fetchSystemInfo, {
    refreshInterval: REFRESH_INTERVAL,
    onSuccess: store.setSystemInfo,
  })

  useEffect(() => {
    const timer = window.setInterval(store.tick, 1000)
    return () => window.clearInterval(timer)
  }, [store.tick])

  const requests = [status, summary, sets, jobs, vitals, config, system]
  return {
    isLoading: requests.some(request => request.isLoading),
    error: requests.find(request => request.error)?.error ?? null,
  }
}
