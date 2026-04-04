import type { DaemonStatus, RunnerSet, JobRecord } from './types'

const now = new Date()
const ago = (s: number) => new Date(now.getTime() - s * 1000)

export const daemonStatus: DaemonStatus = {
  version: '0.2.1',
  commitSha: 'a3f2c1d',
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

export const recentJobs: JobRecord[] = [
  { id: '1', runnerName: 'docker-linux-arm64-e1a3d', runnerSetName: 'docker-linux-arm64', result: 'running', startedAt: ago(482), completedAt: null },
  { id: '2', runnerName: 'macos-tart-arm64-b7d1a',   runnerSetName: 'macos-tart-arm64',   result: 'running', startedAt: ago(331), completedAt: null },
  { id: '3', runnerName: 'macos-tart-arm64-a3f2c',   runnerSetName: 'macos-tart-arm64',   result: 'running', startedAt: ago(134), completedAt: null },
  { id: '4', runnerName: 'docker-linux-arm64-f5b2e', runnerSetName: 'docker-linux-arm64', result: 'success', startedAt: ago(820),  completedAt: ago(618) },
  { id: '5', runnerName: 'macos-tart-arm64-h2d5g',   runnerSetName: 'macos-tart-arm64',   result: 'success', startedAt: ago(1100), completedAt: ago(949) },
  { id: '6', runnerName: 'docker-linux-arm64-g3c4f', runnerSetName: 'docker-linux-arm64', result: 'failure', startedAt: ago(1240), completedAt: ago(1232) },
  { id: '7', runnerName: 'macos-tart-arm64-c9e2b',   runnerSetName: 'macos-tart-arm64',   result: 'success', startedAt: ago(1540), completedAt: ago(1495) },
  { id: '8', runnerName: 'docker-linux-arm64-i7e6h', runnerSetName: 'docker-linux-arm64', result: 'success', startedAt: ago(2100), completedAt: ago(2062) },
  { id: '9', runnerName: 'macos-tart-arm64-j8f3k',   runnerSetName: 'macos-tart-arm64',   result: 'success', startedAt: ago(2400), completedAt: ago(2289) },
  { id: '10', runnerName: 'docker-linux-arm64-k2m9p', runnerSetName: 'docker-linux-arm64', result: 'failure', startedAt: ago(3010), completedAt: ago(3002) },
]
