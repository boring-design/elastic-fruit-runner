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

export function shortName(name: string | null | undefined): string {
  // Guard against missing names — the API can omit runnerName for orphan
  // completion records (job completed but daemon never saw the start event).
  // Crashing the entire dashboard for a single missing field is far worse
  // than rendering a placeholder. See issue #69.
  if (!name) return '—'
  const parts = name.split('-')
  const suffix = parts[parts.length - 1]
  const prefix = parts.slice(0, -1).join('-')
  const max = 20
  const trimmed = prefix.length > max ? '...' + prefix.slice(-(max - 3)) : prefix
  return prefix ? `${trimmed}-${suffix}` : suffix
}
