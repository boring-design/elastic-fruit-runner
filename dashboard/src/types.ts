export type RunnerState = 'preparing' | 'idle' | 'busy' | 'unknown'
export type Backend = 'tart' | 'docker' | 'unknown'
export type JobResult = 'success' | 'failure' | 'canceled' | 'running' | 'unknown'

export interface Runner {
  name: string
  state: RunnerState
  since: Date
}

export interface RunnerSet {
  name: string
  backend: Backend
  image: string
  labels: string[]
  maxRunners: number
  scope: string
  connected: boolean
  runners: Runner[]
}

export interface JobRecord {
  id: string
  runnerName: string
  runnerSetName: string
  result: JobResult
  startedAt: Date
  completedAt: Date | null
}

export interface DaemonStatus {
  buildInfo: BuildInfo | null
  startedAt: Date
  // null while runner set data has not been fetched yet (loading state)
  githubConnected: boolean | null
  idleTimeout: number
}

export interface BuildInfo {
  goVersion: string
  path: string
  main: Module | null
  deps: Module[]
  settings: BuildSetting[]
}

export interface Module {
  path: string
  version: string
  sum: string
  replace: Module | null
}

export interface BuildSetting {
  key: string
  value: string
}

export interface MachineVitals {
  cpuUsagePercent: number
  memoryUsagePercent: number
  diskUsagePercent: number
  temperatureCelsius: number
}
