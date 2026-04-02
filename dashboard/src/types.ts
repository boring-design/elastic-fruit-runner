export type RunnerState = 'preparing' | 'idle' | 'busy'
export type Backend = 'tart' | 'docker'
export type JobResult = 'success' | 'failure' | 'running'

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
  githubConnected: boolean
  idleTimeout: number
}
