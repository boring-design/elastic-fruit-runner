import { create } from 'zustand'
import { devtools } from 'zustand/middleware'
import type { DaemonStatus, RunnerSet, JobRecord } from '../types'

export interface DashboardState {
  daemonStatus: DaemonStatus | null
  runnerSets: RunnerSet[]
  recentJobs: JobRecord[]
  now: Date

  setDaemonStatus: (status: DaemonStatus) => void
  setRunnerSets: (sets: RunnerSet[]) => void
  setRecentJobs: (jobs: JobRecord[]) => void
  tick: () => void
}

export const useDashboardStore = create<DashboardState>()(
  devtools(
    (set) => ({
      daemonStatus: null,
      runnerSets: [],
      recentJobs: [],
      now: new Date(),

      setDaemonStatus: (status) => set({ daemonStatus: status }, false, 'setDaemonStatus'),
      setRunnerSets: (sets) => set({ runnerSets: sets }, false, 'setRunnerSets'),
      setRecentJobs: (jobs) => set({ recentJobs: jobs }, false, 'setRecentJobs'),
      tick: () => set({ now: new Date() }, false, 'tick'),
    }),
    { name: 'dashboard' },
  ),
)
