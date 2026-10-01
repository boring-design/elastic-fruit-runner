export interface WizardTarget {
  kind: 'org' | 'repo'
  name: string
}

export type WizardAuth =
  | { kind: 'pat'; token: string }
  | { kind: 'app'; clientId: string; installationId: string; privateKeyPath: string }

export interface WizardRunnerSet {
  name: string
  backend: string
  image: string
  labels: string[]
  maxRunners: number
  platform: string
}

export interface WizardConfig {
  target: WizardTarget
  auth: WizardAuth
  runnerGroup: string
  runnerSets: WizardRunnerSet[]
}

// Every string scalar is written as a double quoted YAML string.
function quote(value: string) {
  return JSON.stringify(value)
}

// installation_id must be a plain integer. Anything else is quoted so the
// server validation reports it instead of the YAML parser.
function integerOrQuoted(value: string) {
  const text = value.trim()
  return /^\d+$/.test(text) ? text : quote(text)
}

export function buildConfigYAML(form: WizardConfig): string {
  const isOrg = form.target.kind === 'org'
  const lines: string[] = []
  lines.push(isOrg ? 'orgs:' : 'repos:')
  lines.push(`  - ${isOrg ? 'org' : 'repo'}: ${quote(form.target.name.trim())}`)
  lines.push('    auth:')
  if (form.auth.kind === 'pat') {
    lines.push(`      pat_token: ${quote(form.auth.token)}`)
  } else {
    lines.push('      github_app:')
    lines.push(`        client_id: ${quote(form.auth.clientId.trim())}`)
    lines.push(`        installation_id: ${integerOrQuoted(form.auth.installationId)}`)
    lines.push(`        private_key_path: ${quote(form.auth.privateKeyPath.trim())}`)
  }
  if (isOrg) {
    lines.push(`    runner_group: ${quote(form.runnerGroup.trim() || 'Default')}`)
  }
  lines.push('    runner_sets:')
  for (const set of form.runnerSets) {
    lines.push(`      - name: ${quote(set.name.trim())}`)
    lines.push(`        backend: ${quote(set.backend)}`)
    lines.push(`        image: ${quote(set.image.trim())}`)
    lines.push(`        labels: [${set.labels.map(label => quote(label.trim())).join(', ')}]`)
    lines.push(`        max_runners: ${Number.isInteger(set.maxRunners) ? set.maxRunners : 0}`)
    if (set.platform.trim()) {
      lines.push(`        platform: ${quote(set.platform.trim())}`)
    }
  }
  lines.push('')
  lines.push('idle_timeout: 15m')
  lines.push('log_level: info')
  return lines.join('\n') + '\n'
}

// maskToken hides the PAT in a preview. The YAML holds the token as a quoted
// string, so the quoted form is what gets replaced.
export function maskToken(yaml: string, token: string): string {
  if (!token) return yaml
  return yaml.split(quote(token)).join('"********"')
}
