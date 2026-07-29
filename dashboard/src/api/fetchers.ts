import type {
  Backend,
  BuildInfo,
  ConfigStatus,
  ConfigSyncState,
  DaemonStatus,
  DashboardSummary,
  JobRecord,
  MachineVitals,
  Module,
  Runner,
  RunnerSet,
  ResourceSample,
  JobLog,
  SessionState,
  SystemInfo,
} from '../types'

const API_BASE = import.meta.env.VITE_API_BASE ?? ''

async function rpc<T>(
  method: string,
  body: Record<string, unknown> = {},
  csrfToken = '',
): Promise<T> {
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    'Connect-Protocol-Version': '1',
  }
  if (csrfToken) headers['X-CSRF-Token'] = csrfToken

  const response = await fetch(`${API_BASE}/controlplane.v1.ControlPlaneService/${method}`, {
    method: 'POST',
    credentials: 'same-origin',
    headers,
    body: JSON.stringify(body),
  })
  if (!response.ok) {
    let message = `${response.status} ${response.statusText}`
    try {
      const error = await response.json() as { message?: string }
      if (error.message) message = error.message
    } catch {
      // Keep the HTTP status when the response is not JSON.
    }
    throw new Error(message)
  }
  return response.json()
}

export async function fetchSession(): Promise<SessionState> {
  const data = await rpc<{
    setupRequired?: boolean
    authenticated?: boolean
    csrfToken?: string
  }>('GetSession')
  return {
    setupRequired: data.setupRequired ?? false,
    authenticated: data.authenticated ?? false,
    csrfToken: data.csrfToken ?? '',
  }
}

export async function setupAdmin(setupCode: string, password: string): Promise<string> {
  const data = await rpc<{ csrfToken?: string }>('SetupAdmin', { setupCode, password })
  return data.csrfToken ?? ''
}

export async function login(password: string): Promise<string> {
  const data = await rpc<{ csrfToken?: string }>('Login', { password })
  return data.csrfToken ?? ''
}

export async function logout(csrfToken: string): Promise<void> {
  await rpc<Record<string, never>>('Logout', {}, csrfToken)
}

interface ModuleResponse {
  path?: string
  version?: string
  sum?: string
  replace?: ModuleResponse
}

interface BuildInfoResponse {
  goVersion?: string
  path?: string
  main?: ModuleResponse
  deps?: ModuleResponse[]
  settings?: Array<{ key?: string; value?: string }>
}

export async function fetchDaemonStatus(): Promise<DaemonStatus> {
  const data = await rpc<{
    buildInfo?: BuildInfoResponse
    startedAt: string
    idleTimeoutSeconds: number
  }>('GetServiceInfo')
  return {
    buildInfo: data.buildInfo ? toBuildInfo(data.buildInfo) : null,
    startedAt: new Date(data.startedAt),
    idleTimeout: data.idleTimeoutSeconds,
  }
}

function toBuildInfo(data: BuildInfoResponse): BuildInfo {
  return {
    goVersion: data.goVersion ?? '',
    path: data.path ?? '',
    main: data.main ? toModule(data.main) : null,
    deps: (data.deps ?? []).map(toModule),
    settings: (data.settings ?? []).map(setting => ({
      key: setting.key ?? '',
      value: setting.value ?? '',
    })),
  }
}

function toModule(data: ModuleResponse): Module {
  return {
    path: data.path ?? '',
    version: data.version ?? '',
    sum: data.sum ?? '',
    replace: data.replace ? toModule(data.replace) : null,
  }
}

export async function fetchDashboardSummary(): Promise<DashboardSummary> {
  const data = await rpc<Partial<DashboardSummary>>('GetDashboardSummary')
  return {
    runnerSetCount: data.runnerSetCount ?? 0,
    preparingRunnerCount: data.preparingRunnerCount ?? 0,
    idleRunnerCount: data.idleRunnerCount ?? 0,
    busyRunnerCount: data.busyRunnerCount ?? 0,
    runningJobCount: data.runningJobCount ?? 0,
    failedJobCount: data.failedJobCount ?? 0,
    completedJobCount: data.completedJobCount ?? 0,
    githubConnected: data.githubConnected ?? false,
  }
}

const RUNNER_STATE_MAP: Record<string, Runner['state']> = {
  RUNNER_STATE_PREPARING: 'preparing',
  RUNNER_STATE_IDLE: 'idle',
  RUNNER_STATE_BUSY: 'busy',
}

const BACKEND_MAP: Record<string, Backend> = {
  BACKEND_TART: 'tart',
  BACKEND_DOCKER: 'docker',
}

export async function fetchRunnerSets(): Promise<RunnerSet[]> {
  const data = await rpc<{
    runnerSets?: Array<{
      name: string
      backend: string
      image: string
      labels?: string[]
      maxRunners: number
      scope: string
      connected: boolean
      runners?: Array<{ name: string; state: string; since: string }>
    }>
  }>('ListRunnerSets')
  return (data.runnerSets ?? []).map(runnerSet => ({
    name: runnerSet.name,
    backend: BACKEND_MAP[runnerSet.backend] ?? 'unknown',
    image: runnerSet.image,
    labels: runnerSet.labels ?? [],
    maxRunners: runnerSet.maxRunners,
    scope: runnerSet.scope,
    connected: runnerSet.connected,
    runners: (runnerSet.runners ?? []).map(runner => ({
      name: runner.name,
      state: RUNNER_STATE_MAP[runner.state] ?? 'unknown',
      since: new Date(runner.since),
    })),
  }))
}

const JOB_RESULT_MAP: Record<string, JobRecord['result']> = {
  JOB_RESULT_RUNNING: 'running',
  JOB_RESULT_SUCCESS: 'success',
  JOB_RESULT_FAILURE: 'failure',
  JOB_RESULT_CANCELED: 'canceled',
}

export async function fetchRecentJobs(): Promise<JobRecord[]> {
  const page = await fetchJobs({})
  return page.jobs
}

interface JobResponse {
  id: string
  runnerName: string
  runnerSetName: string
  result: string
  startedAt: string
  completedAt?: string
  owner?: string
  repository?: string
  workflowRef?: string
  displayName?: string
  workflowRunId?: number
  eventName?: string
  labels?: string[]
  queuedAt?: string
  scaleSetAssignedAt?: string
  runnerAssignedAt?: string
  backend?: string
  actionsUrl?: string
}

export async function fetchJobs(filters: {
  status?: string
  runnerSet?: string
  repository?: string
  workflow?: string
  cursor?: string
  pageSize?: number
}): Promise<{ jobs: JobRecord[]; nextCursor: string }> {
  const data = await rpc<{
    jobRecords?: JobResponse[]
    nextCursor?: string
  }>('ListJobRecords', filters)
  return {
    jobs: (data.jobRecords ?? []).map(toJob),
    nextCursor: data.nextCursor ?? '',
  }
}

export async function fetchJobDetail(id: string): Promise<JobRecord> {
  const data = await rpc<{ job: JobResponse }>('GetJobDetail', { id })
  return toJob(data.job)
}

export async function fetchJobLogs(jobId: string, afterSequence = 0): Promise<{ lines: JobLog[]; nextSequence: number }> {
  const data = await rpc<{
    lines?: Array<{ sequence: number; recordedAt: string; text: string }>
    nextSequence?: number
  }>('GetJobLogs', { jobId, afterSequence, pageSize: 500 })
  return {
    lines: (data.lines ?? []).map(line => ({
      sequence: line.sequence,
      recordedAt: new Date(line.recordedAt),
      text: line.text,
    })),
    nextSequence: data.nextSequence ?? afterSequence,
  }
}

export async function fetchJobResources(jobId: string): Promise<ResourceSample[]> {
  const data = await rpc<{ samples?: ResourceSampleResponse[] }>('GetJobResourceSamples', { jobId })
  return (data.samples ?? []).map(toResourceSample)
}

export async function fetchHostResources(from: Date, to: Date): Promise<{ samples: ResourceSample[]; earliestAt: Date | null }> {
  const data = await rpc<{ samples?: ResourceSampleResponse[]; earliestAt?: string }>(
    'GetHostResourceSamples',
    { from: from.toISOString(), to: to.toISOString() },
  )
  return {
    samples: (data.samples ?? []).map(toResourceSample),
    earliestAt: data.earliestAt ? new Date(data.earliestAt) : null,
  }
}

interface ResourceSampleResponse {
  recordedAt: string
  source?: string
  accuracy?: string
  cpuPercent?: number
  memoryUsedBytes?: number
  memoryAvailableBytes?: number
  diskUsedBytes?: number
  diskAvailableBytes?: number
  diskReadBytes?: number
  diskWriteBytes?: number
  networkReceiveBytes?: number
  networkSendBytes?: number
  loadOne?: number
  temperatureCelsius?: number
}

function toResourceSample(sample: ResourceSampleResponse): ResourceSample {
  return {
    recordedAt: new Date(sample.recordedAt),
    source: sample.source ?? '',
    accuracy: sample.accuracy === 'RESOURCE_ACCURACY_ESTIMATE' ? 'estimate' : 'exact',
    cpuPercent: sample.cpuPercent ?? 0,
    memoryUsedBytes: sample.memoryUsedBytes ?? 0,
    memoryAvailableBytes: sample.memoryAvailableBytes ?? 0,
    diskUsedBytes: sample.diskUsedBytes ?? 0,
    diskAvailableBytes: sample.diskAvailableBytes ?? 0,
    diskReadBytes: sample.diskReadBytes ?? 0,
    diskWriteBytes: sample.diskWriteBytes ?? 0,
    networkReceiveBytes: sample.networkReceiveBytes ?? 0,
    networkSendBytes: sample.networkSendBytes ?? 0,
    loadOne: sample.loadOne ?? 0,
    temperatureCelsius: sample.temperatureCelsius ?? 0,
  }
}

function toJob(job: JobResponse): JobRecord {
  return {
    id: job.id,
    runnerName: job.runnerName,
    runnerSetName: job.runnerSetName,
    result: JOB_RESULT_MAP[job.result] ?? (job.completedAt ? 'failure' : 'running'),
    startedAt: new Date(job.startedAt),
    completedAt: job.completedAt ? new Date(job.completedAt) : null,
    owner: job.owner ?? '',
    repository: job.repository ?? '',
    workflowRef: job.workflowRef ?? '',
    displayName: job.displayName ?? '',
    workflowRunId: job.workflowRunId ?? 0,
    eventName: job.eventName ?? '',
    labels: job.labels ?? [],
    queuedAt: job.queuedAt ? new Date(job.queuedAt) : null,
    scaleSetAssignedAt: job.scaleSetAssignedAt ? new Date(job.scaleSetAssignedAt) : null,
    runnerAssignedAt: job.runnerAssignedAt ? new Date(job.runnerAssignedAt) : null,
    backend: BACKEND_MAP[job.backend ?? ''] ?? 'unknown',
    actionsURL: job.actionsUrl ?? '',
  }
}

export async function fetchMachineVitals(): Promise<MachineVitals> {
  const data = await rpc<Partial<MachineVitals>>('GetMachineVitals')
  return {
    cpuUsagePercent: data.cpuUsagePercent ?? 0,
    memoryUsagePercent: data.memoryUsagePercent ?? 0,
    diskUsagePercent: data.diskUsagePercent ?? 0,
    temperatureCelsius: data.temperatureCelsius ?? 0,
  }
}

const CONFIG_STATE_MAP: Record<string, ConfigSyncState> = {
  CONFIG_SYNC_STATE_IN_SYNC: 'in_sync',
  CONFIG_SYNC_STATE_RESTART_REQUIRED: 'restart_required',
  CONFIG_SYNC_STATE_DISK_INVALID: 'disk_invalid',
}

export async function fetchConfigStatus(): Promise<ConfigStatus> {
  const data = await rpc<{
    path?: string
    activeHash?: string
    diskHash?: string
    state?: string
    diskModifiedAt?: string
    activeLoadedAt?: string
    validationErrors?: string[]
    activeYaml?: string
    diskYaml?: string
  }>('GetConfigStatus')
  return {
    path: data.path ?? '',
    activeHash: data.activeHash ?? '',
    diskHash: data.diskHash ?? '',
    state: CONFIG_STATE_MAP[data.state ?? ''] ?? 'unknown',
    diskModifiedAt: data.diskModifiedAt ? new Date(data.diskModifiedAt) : null,
    activeLoadedAt: data.activeLoadedAt ? new Date(data.activeLoadedAt) : new Date(0),
    validationErrors: data.validationErrors ?? [],
    activeYAML: data.activeYaml ?? '',
    diskYAML: data.diskYaml ?? '',
  }
}

export async function fetchSystemInfo(): Promise<SystemInfo> {
  const data = await rpc<Partial<SystemInfo>>('GetSystemInfo')
  return {
    os: data.os ?? '',
    arch: data.arch ?? '',
    goVersion: data.goVersion ?? '',
    databasePath: data.databasePath ?? '',
    databaseSizeBytes: data.databaseSizeBytes ?? 0,
  }
}
