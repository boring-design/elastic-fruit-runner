import type { BackendCheck } from '../types'
import type { WizardRunnerSet } from './buildConfigYAML'

export interface RunnerSetPreset extends WizardRunnerSet {
  available: boolean
  unavailableReason: string
}

const dockerImage = 'ghcr.io/actions-runner-controller/actions-runner-controller/actions-runner-dind:latest'
const macImage = 'ghcr.io/cirruslabs/macos-tahoe-xcode:26.3'

// runnerSetPresets picks starting runner sets for the host the daemon runs on.
export function runnerSetPresets(docker: BackendCheck, tart: BackendCheck): RunnerSetPreset[] {
  const hostOS = docker.hostOS || tart.hostOS
  const hostArch = docker.hostArch || tart.hostArch || 'unknown'
  const dockerPreset = (name: string, arch: string, platform: string): RunnerSetPreset => ({
    name,
    backend: 'docker',
    image: dockerImage,
    labels: ['self-hosted', 'linux', arch],
    maxRunners: 4,
    platform,
    available: docker.available,
    unavailableReason: docker.error,
  })
  if (hostOS === 'darwin' && hostArch === 'arm64') {
    return [
      {
        name: 'efr-macos-arm64',
        backend: 'tart',
        image: macImage,
        labels: ['self-hosted', 'macos', 'arm64'],
        maxRunners: 2,
        platform: '',
        available: tart.available,
        unavailableReason: tart.error,
      },
      dockerPreset('efr-linux-arm64', 'arm64', 'linux/arm64'),
    ]
  }
  return [dockerPreset(`efr-linux-${hostArch}`, hostArch, '')]
}
