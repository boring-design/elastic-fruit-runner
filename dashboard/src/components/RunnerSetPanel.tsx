import type { Runner, RunnerSet } from '../types'
import { elapsed, fmtDuration, shortName } from '../utils'

function StateLabel({ state }: { state: Runner['state'] }) {
  const map: Record<Runner['state'], { label: string; color: string }> = {
    busy:      { label: 'BUSY', color: '#f0f0f0' },
    idle:      { label: 'IDLE', color: '#888' },
    preparing: { label: 'PREP', color: '#aaa' },
    unknown:   { label: '???',  color: '#666' },
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
      <span style={{ color: '#bbb', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
        {shortName(runner.name)}
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
    <div style={{ marginBottom: 28 }}>
      {/* Set header */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'baseline', marginBottom: 6 }}>
        <span style={{ fontWeight: 700, fontSize: 13, letterSpacing: '0.04em' }}>
          {rs.name}
        </span>
        <span style={{ color: '#666', fontSize: 10, letterSpacing: '0.12em' }}>
          {rs.backend.toUpperCase()} · {count}/{rs.maxRunners}
        </span>
      </div>

      {/* Image */}
      <div style={{ color: '#555', fontSize: 10, marginBottom: 10, letterSpacing: '0.04em', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
        {rs.image}
      </div>

      {/* Progress bar */}
      <div style={{ marginBottom: 8 }}>
        <div className="progress-track">
          <div className="progress-fill" style={{ width: `${util * 100}%` }} />
        </div>
      </div>

      {/* State counts */}
      <div style={{ display: 'flex', gap: 16, marginBottom: 12, fontSize: 10, letterSpacing: '0.12em' }}>
        {busyCount > 0 && <span style={{ color: '#f0f0f0' }}>{busyCount} BUSY</span>}
        {idleCount > 0 && <span style={{ color: '#888' }}>{idleCount} IDLE</span>}
        {prepCount > 0 && <span style={{ color: '#aaa' }}>{prepCount} PREP</span>}
        {unknownCount > 0 && <span style={{ color: '#666' }}>{unknownCount} ???</span>}
        {count === 0 && <span style={{ color: '#444' }}>NO RUNNERS</span>}
      </div>

      {/* Column headers */}
      <div style={{ display: 'grid', gridTemplateColumns: '1fr 64px 64px', gap: 10, paddingBottom: 5, marginBottom: 4, borderBottom: '1px solid #242424', fontSize: 9, color: '#444', letterSpacing: '0.15em', textTransform: 'uppercase' }}>
        <span>Runner</span>
        <span>State</span>
        <span style={{ textAlign: 'right' }}>Duration</span>
      </div>

      {/* Runners */}
      {rs.runners.map(r => (
        <RunnerRow key={r.name} runner={r} now={now} />
      ))}
      {rs.runners.length === 0 && (
        <div style={{ color: '#444', fontSize: 12, padding: '8px 0' }}>— no active runners —</div>
      )}
    </div>
  )
}
