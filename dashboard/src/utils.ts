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

export function shortName(name: string, max = 32): string {
  if (name.length <= max) return name
  const parts = name.split('-')
  if (parts.length < 2) return `${name.slice(0, max - 1)}…`
  const suffix = parts[parts.length - 1]
  const prefix = parts.slice(0, -1).join('-')
  const room = max - suffix.length - 2
  if (room <= 0) return `…${suffix}`
  return `${prefix.slice(0, room)}…${suffix}`
}
