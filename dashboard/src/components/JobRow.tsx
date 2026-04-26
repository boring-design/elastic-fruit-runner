import type { JobRecord } from '../types'
import { elapsed, fmtDuration, shortName } from '../utils'

export function JobRow({ job, now }: { job: JobRecord; now: Date }) {
  const isRunning = job.result === 'running'
  const duration = job.completedAt
    ? Math.floor((job.completedAt.getTime() - job.startedAt.getTime()) / 1000)
    : elapsed(job.startedAt, now)

  const resultEl = isRunning
    ? <span style={{ color: '#aaa', fontSize: 10, letterSpacing: '0.08em' }} className="pulse">RUN</span>
    : job.result === 'success'
    ? <span style={{ color: '#f0f0f0', fontSize: 10, letterSpacing: '0.08em' }}>OK</span>
    : job.result === 'canceled'
    ? <span style={{ color: '#ff9500', fontSize: 10, letterSpacing: '0.08em' }}>CXL</span>
    : <span style={{ color: '#ff3b30', fontSize: 10, letterSpacing: '0.08em' }}>FAIL</span>

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
