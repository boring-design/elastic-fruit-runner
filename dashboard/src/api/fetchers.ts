import type { DaemonStatus, RunnerSet, JobRecord } from '../types'
import { daemonStatus, runnerSets, recentJobs } from '../mock'

export async function fetchDaemonStatus(): Promise<DaemonStatus> {
  return daemonStatus
}

export async function fetchRunnerSets(): Promise<RunnerSet[]> {
  return runnerSets
}

export async function fetchRecentJobs(): Promise<JobRecord[]> {
  return recentJobs
}
