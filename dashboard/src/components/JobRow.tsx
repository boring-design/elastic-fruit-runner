import type { JobRecord } from '../types'
import { actionsRunURL, elapsed, fmtDuration, shortName } from '../utils'

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

  const runUrl = actionsRunURL(job.repository, job.workflowRunId)
  const runnerColor = isRunning ? '#f0f0f0' : '#888'
  const linkColor = isRunning ? '#f0f0f0' : '#aaa'

  // Build the context line as: "<repository> / <workflow_name>" with graceful
  // omission when fields are missing. The runner name is rendered separately
  // below so users can correlate logs even when GitHub-side context is absent.
  const contextParts = [job.repository, job.workflowName].filter(part => part.length > 0)
  const contextLine = contextParts.join(' / ')
  const tooltip = runUrl
    ? `Open GitHub Actions run for job ${job.id}`
    : 'GitHub Actions run details unavailable for this job'

  const runnerLabel = shortName(job.runnerName)
  const runnerEl = runUrl
    ? (
      <a
        href={runUrl}
        target="_blank"
        rel="noopener noreferrer"
        title={tooltip}
        className="job-link job-context-runner"
        style={{ color: linkColor }}
      >
        {runnerLabel}
      </a>
    )
    : (
      <span className="job-context-runner" style={{ color: runnerColor }}>
        {runnerLabel}
      </span>
    )

  return (
    <div className="job-row" style={{ opacity: isRunning ? 1 : 0.7 }}>
      <div className="job-context">
        {contextLine && (
          <span className="job-context-line" title={contextLine}>
            {contextLine}
          </span>
        )}
        {runnerEl}
      </div>
      {resultEl}
      <span style={{ color: '#666', textAlign: 'right', fontVariantNumeric: 'tabular-nums' }}>
        {fmtDuration(duration)}
      </span>
    </div>
  )
}
