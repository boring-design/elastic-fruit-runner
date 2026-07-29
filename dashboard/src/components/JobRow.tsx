import type { JobRecord } from '../types'
import { elapsed, fmtDuration, shortName } from '../utils'

export function JobRow({ job, now }: { job: JobRecord; now: Date }) {
  const isRunning = job.result === 'running'
  // Tolerate malformed timestamps from the API rather than crashing the
  // whole dashboard. NaN propagates safely through fmtDuration.
  const startedMs = job.startedAt instanceof Date ? job.startedAt.getTime() : NaN
  const completedMs = job.completedAt instanceof Date ? job.completedAt.getTime() : NaN
  const duration = !Number.isNaN(completedMs)
    ? Math.max(0, Math.floor((completedMs - startedMs) / 1000))
    : !Number.isNaN(startedMs)
    ? elapsed(job.startedAt, now)
    : 0

  const resultEl = isRunning
    ? <span style={{ color: '#aaa', fontSize: 10, letterSpacing: '0.08em' }} className="pulse">RUN</span>
    : job.result === 'success'
    ? <span style={{ color: '#f0f0f0', fontSize: 10, letterSpacing: '0.08em' }}>OK</span>
    : job.result === 'canceled'
    ? <span style={{ color: '#ff9500', fontSize: 10, letterSpacing: '0.08em' }}>CXL</span>
    : job.result === 'failure'
    ? <span style={{ color: '#ff3b30', fontSize: 10, letterSpacing: '0.08em' }}>FAIL</span>
    : <span style={{ color: '#888', fontSize: 10, letterSpacing: '0.08em' }} title="Result not recognised by daemon">?</span>

  return (
    <div className="job-row" style={{ opacity: isRunning ? 1 : 0.7 }}>
      <span style={{
        color: isRunning ? '#f0f0f0' : '#888',
        overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap',
      }}>
        {shortName(job.runnerName)}
      </span>
      {resultEl}
      <span style={{ color: '#666', textAlign: 'right', fontVariantNumeric: 'tabular-nums' }}>
        {fmtDuration(duration)}
      </span>
    </div>
  )
}
