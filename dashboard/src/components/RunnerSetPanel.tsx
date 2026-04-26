import type { Runner, RunnerSet } from '../types'
import { elapsed, fmtDuration } from '../utils'

function StateLabel({ state }: { state: Runner['state'] }) {
  const map: Record<Runner['state'], { label: string; color: string }> = {
    busy:      { label: 'BUSY', color: '#f0f0f0' },
    idle:      { label: 'IDLE', color: '#888' },
    preparing: { label: 'PREP', color: '#aaa' },
    unknown:   { label: 'UNKNOWN', color: '#666' },
  }
  const { label, color } = map[state]
  return (
    <span className="badge" style={{ color, borderColor: color }}>
      {label}
    </span>
  )
}

function RunnerRow({ runner, now }: { runner: Runner; now: Date }) {
  const sec = elapsed(runner.since, now)
  return (
    <div className="runner-row">
      <span
        className="ellipsis"
        title={runner.name}
        style={{ color: '#bbb', minWidth: 0 }}
      >
        {runner.name}
      </span>
      <StateLabel state={runner.state} />
      <span style={{ color: '#888', textAlign: 'right', fontVariantNumeric: 'tabular-nums' }}>
        {fmtDuration(sec)}
      </span>
    </div>
  )
}

export function RunnerSetPanel({ rs, now }: { rs: RunnerSet; now: Date }) {
  const count = rs.runners.length
  const util = count / rs.maxRunners
  const busyCount    = rs.runners.filter(r => r.state === 'busy').length
  const idleCount    = rs.runners.filter(r => r.state === 'idle').length
  const prepCount    = rs.runners.filter(r => r.state === 'preparing').length
  const unknownCount = rs.runners.filter(r => r.state === 'unknown').length

  return (
    <div className="runner-set">
      {/* Set header */}
      <div className="runner-set-header">
        <span
          className="runner-set-name ellipsis"
          title={rs.name}
        >
          {rs.name}
        </span>
        <span className="runner-set-meta">
          {rs.backend.toUpperCase()} · {count}/{rs.maxRunners}
        </span>
      </div>

      {/* Image */}
      <div className="runner-set-image ellipsis" title={rs.image}>
        {rs.image}
      </div>

      {/* Progress bar */}
      <div className="progress-track" style={{ marginBottom: 8 }}>
        <div className="progress-fill" style={{ width: `${util * 100}%` }} />
      </div>

      {/* State counts */}
      <div className="runner-set-states">
        {busyCount > 0 && <span style={{ color: '#f0f0f0' }}>{busyCount} BUSY</span>}
        {idleCount > 0 && <span style={{ color: '#888' }}>{idleCount} IDLE</span>}
        {prepCount > 0 && <span style={{ color: '#aaa' }}>{prepCount} PREP</span>}
        {unknownCount > 0 && <span style={{ color: '#666' }}>{unknownCount} UNKNOWN</span>}
        {count === 0 && <span style={{ color: '#444' }}>NO RUNNERS</span>}
      </div>

      {count > 0 && (
        <>
          {/* Column headers */}
          <div className="runner-row runner-row-head">
            <span>Runner</span>
            <span>State</span>
            <span style={{ textAlign: 'right' }}>Duration</span>
          </div>

          {/* Runners */}
          {rs.runners.map(r => (
            <RunnerRow key={r.name} runner={r} now={now} />
          ))}
        </>
      )}
    </div>
  )
}
