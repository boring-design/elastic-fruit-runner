import type { DaemonStatus, RunnerSet, JobRecord } from './types'

const now = new Date()
const ago = (s: number) => new Date(now.getTime() - s * 1000)

export const daemonStatus: DaemonStatus = {
  buildInfo: {
    goVersion: 'go1.25.7',
    path: 'github.com/boring-design/elastic-fruit-runner/cmd/elastic-fruit-runner',
    main: {
      path: 'github.com/boring-design/elastic-fruit-runner',
      version: 'v0.2.1',
      sum: '',
      replace: null,
    },
    deps: [],
    settings: [
      { key: 'vcs.revision', value: 'a3f2c1d' },
    ],
  },
  startedAt: ago(9252),
  githubConnected: true,
  idleTimeout: 900,
}

export const runnerSets: RunnerSet[] = [
  {
    name: 'macos-tart-arm64',
    backend: 'tart',
    image: 'ghcr.io/cirruslabs/macos-sequoia-xcode:16',
    labels: ['macos', 'arm64', 'xcode-16'],
    maxRunners: 5,
    scope: 'org: acme-corp',
    connected: true,
    runners: [
      { name: 'macos-tart-arm64-a3f2c', state: 'busy',     since: ago(134) },
      { name: 'macos-tart-arm64-b7d1a', state: 'busy',     since: ago(331) },
      { name: 'macos-tart-arm64-c9e2b', state: 'idle',     since: ago(720) },
      { name: 'macos-tart-arm64-d4f8c', state: 'preparing', since: ago(45) },
    ],
  },
  {
    name: 'docker-linux-arm64',
    backend: 'docker',
    image: 'ghcr.io/quipper/actions-runner:2.332.0',
    labels: ['linux', 'arm64'],
    maxRunners: 5,
    scope: 'org: acme-corp',
    connected: true,
    runners: [
      { name: 'docker-linux-arm64-e1a3d', state: 'busy', since: ago(482) },
      { name: 'docker-linux-arm64-f5b2e', state: 'idle', since: ago(202) },
    ],
  },
]

const job = (
  partial: Pick<JobRecord, 'id' | 'runnerName' | 'runnerSetName' | 'result' | 'startedAt' | 'completedAt'> &
    Partial<Pick<JobRecord, 'repository' | 'workflowName' | 'workflowRunId'>>,
): JobRecord => ({
  repository: partial.repository ?? 'boring-design/elastic-fruit-runner',
  workflowName: partial.workflowName ?? 'Unit Test',
  workflowRunId: partial.workflowRunId ?? '1234567890',
  ...partial,
})

export const recentJobs: JobRecord[] = [
  job({ id: '1', runnerName: 'docker-linux-arm64-e1a3d', runnerSetName: 'docker-linux-arm64', result: 'running', startedAt: ago(482), completedAt: null, workflowName: 'Build Image', workflowRunId: '11111111' }),
  job({ id: '2', runnerName: 'macos-tart-arm64-b7d1a',   runnerSetName: 'macos-tart-arm64',   result: 'running', startedAt: ago(331), completedAt: null, workflowName: 'iOS Build', workflowRunId: '22222222' }),
  job({ id: '3', runnerName: 'macos-tart-arm64-a3f2c',   runnerSetName: 'macos-tart-arm64',   result: 'running', startedAt: ago(134), completedAt: null, workflowName: 'iOS Test', workflowRunId: '33333333' }),
  job({ id: '4', runnerName: 'docker-linux-arm64-f5b2e', runnerSetName: 'docker-linux-arm64', result: 'success', startedAt: ago(820),  completedAt: ago(618), workflowName: 'Lint', workflowRunId: '44444444' }),
  job({ id: '5', runnerName: 'macos-tart-arm64-h2d5g',   runnerSetName: 'macos-tart-arm64',   result: 'success', startedAt: ago(1100), completedAt: ago(949) }),
  job({ id: '6', runnerName: 'docker-linux-arm64-g3c4f', runnerSetName: 'docker-linux-arm64', result: 'failure', startedAt: ago(1240), completedAt: ago(1232), workflowName: 'Integration Test', workflowRunId: '66666666' }),
  job({ id: '7', runnerName: 'macos-tart-arm64-c9e2b',   runnerSetName: 'macos-tart-arm64',   result: 'success', startedAt: ago(1540), completedAt: ago(1495) }),
  job({ id: '8', runnerName: 'docker-linux-arm64-i7e6h', runnerSetName: 'docker-linux-arm64', result: 'success', startedAt: ago(2100), completedAt: ago(2062) }),
  job({ id: '9', runnerName: 'macos-tart-arm64-j8f3k',   runnerSetName: 'macos-tart-arm64',   result: 'success', startedAt: ago(2400), completedAt: ago(2289) }),
  job({ id: '10', runnerName: 'docker-linux-arm64-k2m9p', runnerSetName: 'docker-linux-arm64', result: 'failure', startedAt: ago(3010), completedAt: ago(3002), repository: '', workflowName: '', workflowRunId: '' }),
]
