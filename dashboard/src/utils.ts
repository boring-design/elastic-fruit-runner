export function elapsed(since: Date, now: Date): number {
  return Math.floor((now.getTime() - since.getTime()) / 1000)
}

export function fmtDuration(seconds: number): string {
  if (seconds < 60) return `00:${String(seconds).padStart(2, '0')}`
  const m = Math.floor(seconds / 60)
  const s = seconds % 60
  if (m < 60) return `${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`
  const h = Math.floor(m / 60)
  const rm = m % 60
  return `${String(h).padStart(2, '0')}:${String(rm).padStart(2, '0')}:${String(s).padStart(2, '0')}`
}

export function fmtUptime(seconds: number): string {
  const h = Math.floor(seconds / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  const s = seconds % 60
  return `${String(h).padStart(2, '0')}h:${String(m).padStart(2, '0')}m:${String(s).padStart(2, '0')}s`
}

export function shortName(name: string): string {
  const parts = name.split('-')
  const suffix = parts[parts.length - 1]
  const prefix = parts.slice(0, -1).join('-')
  const max = 20
  const trimmed = prefix.length > max ? '...' + prefix.slice(-(max - 3)) : prefix
  return `${trimmed}-${suffix}`
}

/**
 * Parse a runner set scope string emitted by the daemon into a structured form.
 * The daemon currently emits "org: <org>" or "repo: <owner>/<repo>".
 * Returns null when the scope is empty or in an unknown shape so callers can
 * gracefully degrade (rendering plain text instead of a link).
 */
export function parseRunnerSetScope(
  scope: string,
): { kind: 'org'; org: string } | { kind: 'repo'; repo: string } | null {
  if (!scope) return null
  const trimmed = scope.trim()
  const colon = trimmed.indexOf(':')
  if (colon < 0) return null
  const prefix = trimmed.slice(0, colon).trim().toLowerCase()
  const value = trimmed.slice(colon + 1).trim()
  if (!value) return null
  if (prefix === 'org') {
    return { kind: 'org', org: value }
  }
  if (prefix === 'repo') {
    return { kind: 'repo', repo: value }
  }
  return null
}

/**
 * Build the GitHub UI URL where users manage self-hosted runners for a given
 * runner set scope. Returns null when the scope cannot be parsed so the
 * caller can render the runner-set name as plain text.
 */
export function runnerSetGitHubURL(scope: string): string | null {
  const parsed = parseRunnerSetScope(scope)
  if (!parsed) return null
  if (parsed.kind === 'org') {
    return `https://github.com/organizations/${encodeURIComponent(parsed.org)}/settings/actions/runners`
  }
  return `https://github.com/${parsed.repo}/settings/actions/runners`
}

/**
 * Build the GitHub Actions workflow run URL for a job. Returns null when
 * either the repository or workflow run identifier is missing so the row
 * can be rendered without a link.
 */
export function actionsRunURL(repository: string, workflowRunId: string): string | null {
  if (!repository || !workflowRunId) return null
  return `https://github.com/${repository}/actions/runs/${encodeURIComponent(workflowRunId)}`
}
