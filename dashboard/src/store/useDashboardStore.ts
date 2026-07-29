import { create } from 'zustand'
import type {
  ConfigStatus,
  DaemonStatus,
  DashboardSummary,
  JobRecord,
  MachineVitals,
  RunnerSet,
  SystemInfo,
} from '../types'

export interface DashboardState {
  daemonStatus: DaemonStatus | null
  summary: DashboardSummary | null
  runnerSets: RunnerSet[]
  recentJobs: JobRecord[]
  machineVitals: MachineVitals | null
  configStatus: ConfigStatus | null
  systemInfo: SystemInfo | null
  now: Date
  setDaemonStatus: (value: DaemonStatus) => void
  setSummary: (value: DashboardSummary) => void
  setRunnerSets: (value: RunnerSet[]) => void
  setRecentJobs: (value: JobRecord[]) => void
  setMachineVitals: (value: MachineVitals) => void
  setConfigStatus: (value: ConfigStatus) => void
  setSystemInfo: (value: SystemInfo) => void
  tick: () => void
}

export const useDashboardStore = create<DashboardState>()(set => ({
  daemonStatus: null,
  summary: null,
  runnerSets: [],
  recentJobs: [],
  machineVitals: null,
  configStatus: null,
  systemInfo: null,
  now: new Date(),
  setDaemonStatus: daemonStatus => set({ daemonStatus }),
  setSummary: summary => set({ summary }),
  setRunnerSets: runnerSets => set({ runnerSets }),
  setRecentJobs: recentJobs => set({ recentJobs }),
  setMachineVitals: machineVitals => set({ machineVitals }),
  setConfigStatus: configStatus => set({ configStatus }),
  setSystemInfo: systemInfo => set({ systemInfo }),
  tick: () => set({ now: new Date() }),
}))
