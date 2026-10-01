import type { GitHubAuthInput } from '../api/fetchers'
import type { WizardConfig } from './buildConfigYAML'
import type { RunnerSetPreset } from './presets'

// The labels input keeps the raw text so a typed comma is not lost on the same keystroke.
export type RunnerSetForm = Omit<RunnerSetPreset, 'labels'> & { selected: boolean; labelsText: string }

export interface WizardForm {
  targetKind: 'org' | 'repo'
  targetName: string
  authKind: 'pat' | 'app'
  token: string
  clientId: string
  installationId: string
  privateKeyPath: string
  runnerGroup: string
  runnerSets: RunnerSetForm[]
}

export const emptyForm: WizardForm = {
  targetKind: 'org',
  targetName: '',
  authKind: 'pat',
  token: '',
  clientId: '',
  installationId: '',
  privateKeyPath: '',
  runnerGroup: 'Default',
  runnerSets: [],
}

export function toRunnerSetForm(preset: RunnerSetPreset): RunnerSetForm {
  const { labels, ...rest } = preset
  return { ...rest, selected: preset.available, labelsText: labels.join(', ') }
}

export function parseLabels(text: string): string[] {
  return text.split(',').map(label => label.trim()).filter(Boolean)
}

export function toConfig(form: WizardForm): WizardConfig {
  return {
    target: { kind: form.targetKind, name: form.targetName },
    auth: form.authKind === 'pat'
      ? { kind: 'pat', token: form.token }
      : { kind: 'app', clientId: form.clientId, installationId: form.installationId, privateKeyPath: form.privateKeyPath },
    runnerGroup: form.runnerGroup,
    runnerSets: form.runnerSets.filter(set => set.selected).map(set => ({ ...set, labels: parseLabels(set.labelsText) })),
  }
}

export function toAuthInput(form: WizardForm): GitHubAuthInput {
  const input: GitHubAuthInput = {}
  if (form.targetKind === 'org') {
    input.org = form.targetName.trim()
    input.runnerGroup = form.runnerGroup.trim() || 'Default'
  } else {
    input.repo = form.targetName.trim()
  }
  if (form.authKind === 'pat') {
    input.patToken = form.token
  } else {
    input.githubApp = {
      clientId: form.clientId.trim(),
      installationId: Number(form.installationId),
      privateKeyPath: form.privateKeyPath.trim(),
    }
  }
  return input
}
