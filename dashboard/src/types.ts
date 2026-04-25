export type RunnerState = 'preparing' | 'idle' | 'busy' | 'unknown'
export type Backend = 'tart' | 'docker' | 'unknown'
export type JobResult = 'success' | 'failure' | 'canceled' | 'running'

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
  version: string
  commitSha: string
  startedAt: Date
  // null while runner set data has not been fetched yet (loading state)
  githubConnected: boolean | null
  idleTimeout: number
}

export interface MachineVitals {
  cpuUsagePercent: number
  memoryUsagePercent: number
  diskUsagePercent: number
  temperatureCelsius: number
}
