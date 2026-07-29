import type { DaemonStatus, RunnerSet, JobRecord, Runner, MachineVitals, BuildInfo, Module } from '../types'

const API_BASE = import.meta.env.VITE_API_BASE ?? ''

async function rpc<T>(method: string, body: Record<string, unknown> = {}): Promise<T> {
  const res = await fetch(`${API_BASE}/controlplane.v1.ControlPlaneService/${method}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', 'Connect-Protocol-Version': '1' },
    body: JSON.stringify(body),
  })
  if (!res.ok) {
    throw new Error(`RPC ${method} failed: ${res.status} ${res.statusText}`)
  }
  return res.json()
}

const RUNNER_STATE_MAP: Record<string, Runner['state']> = {
  RUNNER_STATE_PREPARING: 'preparing',
  RUNNER_STATE_IDLE: 'idle',
  RUNNER_STATE_BUSY: 'busy',
}

const BACKEND_MAP: Record<string, RunnerSet['backend']> = {
  BACKEND_TART: 'tart',
  BACKEND_DOCKER: 'docker',
}

const JOB_RESULT_MAP: Record<string, JobRecord['result']> = {
  JOB_RESULT_RUNNING: 'running',
  JOB_RESULT_SUCCESS: 'success',
  JOB_RESULT_FAILURE: 'failure',
  JOB_RESULT_CANCELED: 'canceled',
}

interface ServiceInfoResponse {
  buildInfo?: BuildInfoResponse
  startedAt: string
  idleTimeoutSeconds: number
}

interface BuildInfoResponse {
  goVersion?: string
  path?: string
  main?: ModuleResponse
  deps?: ModuleResponse[]
  settings?: Array<{
    key?: string
    value?: string
  }>
}

interface ModuleResponse {
  path?: string
  version?: string
  sum?: string
  replace?: ModuleResponse
}

interface RunnerSetsResponse {
  runnerSets: Array<{
    name: string
    backend: string
    image: string
    labels: string[]
    maxRunners: number
    scope: string
    connected: boolean
    runners: Array<{
      name: string
      state: string
      since: string
    }>
  }>
}

interface JobRecordsResponse {
  jobRecords: Array<{
    id: string
    runnerName: string
    runnerSetName: string
    result: string
    startedAt: string
    completedAt?: string
    repository?: string
    workflowName?: string
    workflowRunId?: string
  }>
}

interface MachineVitalsResponse {
  cpuUsagePercent: number
  memoryUsagePercent: number
  diskUsagePercent: number
  temperatureCelsius: number
}

let cachedRunnerSets: RunnerSet[] = []

export async function fetchDaemonStatus(): Promise<DaemonStatus> {
  const data = await rpc<ServiceInfoResponse>('GetServiceInfo')
  // Return null when runner set data has not been fetched yet so the UI
  // can show a loading indicator instead of a misleading "CONNECTED".
  const githubConnected = cachedRunnerSets.length > 0
    ? cachedRunnerSets.every(rs => rs.connected)
    : null
  return {
    buildInfo: data.buildInfo ? toBuildInfo(data.buildInfo) : null,
    startedAt: new Date(data.startedAt),
    githubConnected,
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

export async function fetchRunnerSets(): Promise<RunnerSet[]> {
  const data = await rpc<RunnerSetsResponse>('ListRunnerSets')
  const sets = (data.runnerSets ?? []).map((rs): RunnerSet => ({
    name: rs.name,
    backend: BACKEND_MAP[rs.backend] ?? 'unknown',
    image: rs.image,
    labels: rs.labels ?? [],
    maxRunners: rs.maxRunners,
    scope: rs.scope,
    connected: rs.connected,
    runners: (rs.runners ?? []).map((r): Runner => ({
      name: r.name,
      state: RUNNER_STATE_MAP[r.state] ?? 'unknown',
      since: new Date(r.since),
    })),
  }))
  cachedRunnerSets = sets
  return sets
}

export async function fetchRecentJobs(): Promise<JobRecord[]> {
  const data = await rpc<JobRecordsResponse>('ListJobRecords')
  return (data.jobRecords ?? []).map((j): JobRecord => ({
    id: j.id,
    runnerName: j.runnerName,
    runnerSetName: j.runnerSetName,
    result: JOB_RESULT_MAP[j.result] ?? (j.completedAt ? 'failure' : 'running'),
    startedAt: new Date(j.startedAt),
    completedAt: j.completedAt ? new Date(j.completedAt) : null,
    repository: j.repository ?? '',
    workflowName: j.workflowName ?? '',
    workflowRunId: j.workflowRunId ?? '',
  }))
}

export async function fetchMachineVitals(): Promise<MachineVitals> {
  return rpc<MachineVitalsResponse>('GetMachineVitals')
}
