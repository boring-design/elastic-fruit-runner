export type RunnerState = 'preparing' | 'idle' | 'busy' | 'unknown'
export type Backend = 'tart' | 'docker' | 'unknown'
export type JobResult = 'success' | 'failure' | 'canceled' | 'running'
export type ConfigSyncState = 'in_sync' | 'restart_required' | 'disk_invalid' | 'unknown'

export interface SessionState {
  setupRequired: boolean
  authenticated: boolean
  csrfToken: string
}

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
  owner?: string
  repository?: string
  workflowRef?: string
  displayName?: string
  workflowRunId?: number
  eventName?: string
  labels?: string[]
  queuedAt?: Date | null
  scaleSetAssignedAt?: Date | null
  runnerAssignedAt?: Date | null
  backend?: Backend
  actionsURL?: string
}

export interface JobLog {
  sequence: number
  recordedAt: Date
  text: string
}

export interface ResourceSample {
  recordedAt: Date
  source: string
  accuracy: 'exact' | 'estimate'
  cpuPercent: number
  memoryUsedBytes: number
  memoryAvailableBytes: number
  diskUsedBytes: number
  diskAvailableBytes: number
  diskReadBytes: number
  diskWriteBytes: number
  networkReceiveBytes: number
  networkSendBytes: number
  loadOne: number
  temperatureCelsius: number
}

export interface DaemonStatus {
  buildInfo: BuildInfo | null
  startedAt: Date
  idleTimeout: number
}

export interface DashboardSummary {
  runnerSetCount: number
  preparingRunnerCount: number
  idleRunnerCount: number
  busyRunnerCount: number
  runningJobCount: number
  failedJobCount: number
  completedJobCount: number
  githubConnected: boolean
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

export interface ConfigStatus {
  path: string
  activeHash: string
  diskHash: string
  state: ConfigSyncState
  diskModifiedAt: Date | null
  activeLoadedAt: Date
  validationErrors: string[]
  activeYAML: string
  diskYAML: string
  restartCommands: string[]
}

export interface ConfigValidationIssue {
  path: string
  message: string
}

export interface ConfigValidation {
  errors: ConfigValidationIssue[]
  warnings: ConfigValidationIssue[]
  normalizedYAML: string
}

export interface ConfigRevision {
  id: number
  createdAt: Date
  source: string
  configHash: string
}

export interface SystemInfo {
  os: string
  arch: string
  goVersion: string
  databasePath: string
  databaseSizeBytes: number
  logPath: string
  logSizeBytes: number
}
