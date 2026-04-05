import { create } from 'zustand'
import { devtools } from 'zustand/middleware'
import type { DaemonStatus, RunnerSet, JobRecord, MachineVitals } from '../types'

export interface DashboardState {
  daemonStatus: DaemonStatus | null
  runnerSets: RunnerSet[]
  recentJobs: JobRecord[]
  machineVitals: MachineVitals | null
  now: Date

  setDaemonStatus: (status: DaemonStatus) => void
  setRunnerSets: (sets: RunnerSet[]) => void
  setRecentJobs: (jobs: JobRecord[]) => void
  setMachineVitals: (vitals: MachineVitals) => void
  tick: () => void
}

export const useDashboardStore = create<DashboardState>()(
  devtools(
    (set) => ({
      daemonStatus: null,
      runnerSets: [],
      recentJobs: [],
      machineVitals: null,
      now: new Date(),

      setDaemonStatus: (status) => set({ daemonStatus: status }, false, 'setDaemonStatus'),
      setRunnerSets: (sets) => set({ runnerSets: sets }, false, 'setRunnerSets'),
      setRecentJobs: (jobs) => set({ recentJobs: jobs }, false, 'setRecentJobs'),
      setMachineVitals: (vitals) => set({ machineVitals: vitals }, false, 'setMachineVitals'),
      tick: () => set({ now: new Date() }, false, 'tick'),
    }),
    { name: 'dashboard' },
  ),
)
