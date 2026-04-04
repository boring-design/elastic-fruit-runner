import { useState, useEffect } from 'react'
import type { Runner, RunnerSet, JobRecord } from './types'
import { daemonStatus, runnerSets, recentJobs } from './mock'
import { PixelPet } from './components/PixelPet'
import { MOOD_LABEL, MOOD_SUBTEXT, type PetMood } from './components/petMood'
import { SystemVitals } from './components/SystemVitals'

// ─── Utilities ────────────────────────────────────────────────────────────────

function elapsed(since: Date, now: Date): number {
  return Math.floor((now.getTime() - since.getTime()) / 1000)
}

function fmtDuration(seconds: number): string {
  if (seconds < 60) return `00:${String(seconds).padStart(2, '0')}`
  const m = Math.floor(seconds / 60)
  const s = seconds % 60
  if (m < 60) return `${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`
  const h = Math.floor(m / 60)
  const rm = m % 60
  return `${String(h).padStart(2, '0')}:${String(rm).padStart(2, '0')}:${String(s).padStart(2, '0')}`
}

function fmtUptime(seconds: number): string {
  const h = Math.floor(seconds / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  const s = seconds % 60
  return `${String(h).padStart(2, '0')}h:${String(m).padStart(2, '0')}m:${String(s).padStart(2, '0')}s`
}

function shortName(name: string): string {
  const parts = name.split('-')
  const suffix = parts[parts.length - 1]
  const prefix = parts.slice(0, -1).join('-')
  const max = 20
  const trimmed = prefix.length > max ? '...' + prefix.slice(-(max - 3)) : prefix
  return `${trimmed}-${suffix}`
}

// ─── Sub-components ───────────────────────────────────────────────────────────

function StateLabel({ state }: { state: Runner['state'] }) {
  const map = {
    busy:     { label: 'BUSY', color: '#f0f0f0' },
    idle:     { label: 'IDLE', color: '#888' },
    preparing:{ label: 'PREP', color: '#aaa' },
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

function RunnerSetPanel({ rs, now }: { rs: RunnerSet; now: Date }) {
  const count = rs.runners.length
  const util = count / rs.maxRunners
  const busyCount  = rs.runners.filter(r => r.state === 'busy').length
  const idleCount  = rs.runners.filter(r => r.state === 'idle').length
  const prepCount  = rs.runners.filter(r => r.state === 'preparing').length

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

function JobRow({ job, now }: { job: JobRecord; now: Date }) {
  const isRunning = job.result === 'running'
  const duration = job.completedAt
    ? Math.floor((job.completedAt.getTime() - job.startedAt.getTime()) / 1000)
    : elapsed(job.startedAt, now)

  const resultEl = isRunning
    ? <span style={{ color: '#aaa', fontSize: 10, letterSpacing: '0.08em' }} className="pulse">RUN</span>
    : job.result === 'success'
    ? <span style={{ color: '#f0f0f0', fontSize: 10, letterSpacing: '0.08em' }}>OK</span>
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

// ─── Main App ─────────────────────────────────────────────────────────────────

export default function App() {
  const [now, setNow] = useState(new Date())

  useEffect(() => {
    const id = setInterval(() => setNow(new Date()), 1000)
    return () => clearInterval(id)
  }, [])

  const uptime      = elapsed(daemonStatus.startedAt, now)
  const allRunners  = runnerSets.flatMap(rs => rs.runners)
  const totalMax    = runnerSets.reduce((s, rs) => s + rs.maxRunners, 0)
  const totalActive = allRunners.length
  const preparing   = allRunners.filter(r => r.state === 'preparing').length
  const idle        = allRunners.filter(r => r.state === 'idle').length
  const busy        = allRunners.filter(r => r.state === 'busy').length
  const utilPct     = Math.round((totalActive / totalMax) * 100)

  const completedJobs = recentJobs.filter(j => j.result !== 'running')
  const successCount  = completedJobs.filter(j => j.result === 'success').length
  const failureCount  = completedJobs.filter(j => j.result === 'failure').length

  const mood: PetMood =
    preparing > 0 ? 'alert' :
    busy > 0      ? 'busy'  :
    idle > 0      ? 'idle'  :
    'sleeping'

  return (
    <div className="app-container">

      {/* ── Header ── */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 20 }}>
        <div style={{ display: 'flex', alignItems: 'baseline', gap: 20 }}>
          <span style={{ fontSize: 15, fontWeight: 700, letterSpacing: '0.06em' }}>
            ELASTIC-FRUIT-RUNNER
          </span>
          <span style={{ color: '#555', fontSize: 11, letterSpacing: '0.08em' }}>
            v{daemonStatus.version} · {daemonStatus.commitSha}
          </span>
        </div>
        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          <span className="pulse" style={{ fontSize: 10, color: daemonStatus.githubConnected ? '#f0f0f0' : '#ff3b30' }}>●</span>
          <span style={{ fontSize: 11, letterSpacing: '0.12em', color: daemonStatus.githubConnected ? '#f0f0f0' : '#ff3b30' }}>
            {daemonStatus.githubConnected ? 'CONNECTED' : 'DISCONNECTED'}
          </span>
        </div>
      </div>

      {/* ── Top status grid ── */}
      <div className="grid-top">

        {/* Cell: Daemon status */}
        <div className="cell">
          <div className="label">STATUS</div>
          <div style={{ display: 'flex', alignItems: 'center', gap: 16, marginBottom: 16 }}>
            <div className="spinner" />
            <div>
              <div style={{ fontSize: 13, fontWeight: 700, letterSpacing: '0.05em', marginBottom: 4, color: '#f0f0f0' }}>
                LISTENING FOR JOBS
              </div>
              <div style={{ fontSize: 9, color: '#555', letterSpacing: '0.1em' }}>
                GITHUB SCALE SET API
              </div>
            </div>
          </div>

          <div style={{ borderTop: '1px solid #1e1e1e', paddingTop: 14, display: 'flex', flexDirection: 'column', gap: 10 }}>
            {/* Scope */}
            <div>
              <div style={{ fontSize: 9, color: '#444', letterSpacing: '0.15em', marginBottom: 3 }}>CONNECTED TO</div>
              {runnerSets.map(rs => (
                <div key={rs.name} style={{ fontSize: 11, color: '#888', letterSpacing: '0.04em' }}>
                  {rs.scope}
                </div>
              )).filter((_, i) => i === 0)}
            </div>

            {/* Auth + Sets */}
            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 10 }}>
              <div>
                <div style={{ fontSize: 9, color: '#444', letterSpacing: '0.15em', marginBottom: 3 }}>AUTH MODE</div>
                <div style={{ fontSize: 11, color: '#888' }}>GITHUB APP</div>
              </div>
              <div>
                <div style={{ fontSize: 9, color: '#444', letterSpacing: '0.15em', marginBottom: 3 }}>RUNNER SETS</div>
                <div style={{ fontSize: 18, fontWeight: 700, color: '#f0f0f0', lineHeight: 1 }}>{runnerSets.length}</div>
              </div>
            </div>

            {/* Last activity */}
            <div>
              <div style={{ fontSize: 9, color: '#444', letterSpacing: '0.15em', marginBottom: 3 }}>LAST JOB</div>
              <div style={{ fontSize: 11, color: '#888' }}>
                {(() => {
                  const latest = recentJobs.find(j => j.completedAt)
                  if (!latest?.completedAt) return '—'
                  const s = elapsed(latest.completedAt, now)
                  return `${fmtDuration(s)} ago`
                })()}
              </div>
            </div>
          </div>
        </div>

        {/* Cell: Capacity */}
        <div className="cell">
          <div className="label">RUNNER CAPACITY</div>

          {/* Overall bar */}
          <div className="progress-track" style={{ marginBottom: 8 }}>
            <div className="progress-fill" style={{ width: `${utilPct}%` }} />
          </div>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'baseline', marginBottom: 4 }}>
            <span className="value-md">{totalActive}</span>
            <span style={{ fontSize: 11, color: '#555' }}>OF {totalMax} MAX</span>
          </div>
          <div style={{ fontSize: 10, color: '#555', letterSpacing: '0.12em', marginBottom: 16 }}>
            OVERALL UTILIZATION {utilPct}% · {totalMax - totalActive} SLOTS FREE
          </div>

          {/* Per-set breakdown */}
          <div style={{ borderTop: '1px solid #1e1e1e', paddingTop: 14 }}>
            <div style={{ fontSize: 9, color: '#444', letterSpacing: '0.15em', marginBottom: 10 }}>PER SET</div>
            {runnerSets.map(rs => {
              const pct = Math.round((rs.runners.length / rs.maxRunners) * 100)
              return (
                <div key={rs.name} style={{ marginBottom: 10 }}>
                  <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 4 }}>
                    <span style={{ fontSize: 10, color: '#888', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', maxWidth: '70%' }}>
                      {rs.name}
                    </span>
                    <span style={{ fontSize: 10, color: '#555', flexShrink: 0 }}>
                      {rs.runners.length}/{rs.maxRunners}
                    </span>
                  </div>
                  <div style={{ height: 4, background: '#1a1a1a', overflow: 'hidden' }}>
                    <div style={{
                      height: '100%', width: `${pct}%`,
                      background: pct >= 80 ? '#ff9500' : '#e8e8e8',
                      transition: 'width 0.6s ease',
                      backgroundImage: 'repeating-linear-gradient(90deg, transparent 0px, transparent 3px, rgba(0,0,0,0.35) 3px, rgba(0,0,0,0.35) 4px)',
                    }} />
                  </div>
                </div>
              )
            })}
          </div>

          {/* Throughput stats */}
          <div style={{ borderTop: '1px solid #1e1e1e', paddingTop: 14, display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 10 }}>
            <div>
              <div style={{ fontSize: 9, color: '#444', letterSpacing: '0.15em', marginBottom: 3 }}>AVG DURATION</div>
              <div style={{ fontSize: 16, fontWeight: 700, color: '#f0f0f0' }}>
                {(() => {
                  const done = recentJobs.filter(j => j.completedAt)
                  if (!done.length) return '—'
                  const avg = done.reduce((s, j) => s + (j.completedAt!.getTime() - j.startedAt.getTime()) / 1000, 0) / done.length
                  return fmtDuration(Math.round(avg))
                })()}
              </div>
            </div>
            <div>
              <div style={{ fontSize: 9, color: '#444', letterSpacing: '0.15em', marginBottom: 3 }}>SUCCESS RATE</div>
              <div style={{ fontSize: 16, fontWeight: 700, color: successCount + failureCount > 0 && failureCount / (successCount + failureCount) > 0.2 ? '#ff9500' : '#f0f0f0' }}>
                {successCount + failureCount > 0
                  ? `${Math.round(successCount / (successCount + failureCount) * 100)}%`
                  : '—'}
              </div>
            </div>
          </div>
        </div>

        {/* Cell: BRAIN — pixel pet + vitals */}
        <div className="cell cell-brain">
          <div className="label">RUNNER BRAIN</div>
          <div style={{ display: 'flex', gap: 16, alignItems: 'flex-start' }}>
            {/* Pixel pet */}
            <div style={{ flexShrink: 0 }}>
              <PixelPet mood={mood} />
              <div style={{ marginTop: 6, fontSize: 8, letterSpacing: '0.1em', color: '#555', textAlign: 'center' }}>
                {mood.toUpperCase()}
              </div>
            </div>
            {/* Status text + uptime */}
            <div style={{ flex: 1, minWidth: 0 }}>
              <div style={{ fontSize: 12, fontWeight: 700, color: '#f0f0f0', marginBottom: 3, letterSpacing: '0.04em' }}>
                {MOOD_LABEL[mood]}
              </div>
              <div style={{ fontSize: 9, color: '#555', letterSpacing: '0.06em', marginBottom: 12 }}>
                {MOOD_SUBTEXT[mood]}
              </div>
              <div style={{ fontSize: 9, color: '#444', letterSpacing: '0.12em', marginBottom: 2 }}>UPTIME</div>
              <div style={{ fontSize: 16, fontWeight: 700, fontVariantNumeric: 'tabular-nums', letterSpacing: '0.02em', marginBottom: 10 }}>
                {fmtUptime(uptime)}
              </div>
              <div style={{ fontSize: 9, color: '#333', letterSpacing: '0.1em' }}>
                IDLE TIMEOUT {Math.floor(daemonStatus.idleTimeout / 60)}M
              </div>
            </div>
          </div>
          {/* System vitals */}
          <div className="brain-vitals" style={{ borderTop: '1px solid #1e1e1e', marginTop: 14, paddingTop: 14 }}>
            <div className="label" style={{ marginBottom: 10 }}>SYSTEM VITALS</div>
            <SystemVitals />
          </div>
        </div>
      </div>

      {/* ── Stats row ── */}
      <div className="grid-stats">
        {[
          { label: 'PREPARING', value: preparing,    color: '#aaa' },
          { label: 'IDLE',      value: idle,         color: '#888' },
          { label: 'BUSY',      value: busy,         color: '#f0f0f0' },
          { label: 'COMPLETED', value: successCount, color: '#f0f0f0' },
          { label: 'FAILED',    value: failureCount, color: failureCount > 0 ? '#ff3b30' : '#444' },
        ].map((stat) => (
          <div key={stat.label} className="cell">
            <div className="label">{stat.label}</div>
            <div className="value-md" style={{ color: stat.color }}>
              {String(stat.value).padStart(2, '0')}
            </div>
          </div>
        ))}
      </div>

      {/* ── Main area ── */}
      <div className="grid-main">

        {/* Runner Sets */}
        <div className="cell">
          <div className="label" style={{ marginBottom: 22 }}>RUNNER SETS</div>
          {runnerSets.map(rs => (
            <RunnerSetPanel key={rs.name} rs={rs} now={now} />
          ))}
        </div>

        {/* Recent Jobs */}
        <div className="cell">
          <div className="label" style={{ marginBottom: 14 }}>RECENT JOBS</div>
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 54px 58px', gap: 10, paddingBottom: 6, marginBottom: 4, borderBottom: '1px solid #242424', fontSize: 9, color: '#444', letterSpacing: '0.15em', textTransform: 'uppercase' }}>
            <span>Runner</span>
            <span>Result</span>
            <span style={{ textAlign: 'right' }}>Duration</span>
          </div>
          {recentJobs.map(job => (
            <JobRow key={job.id} job={job} now={now} />
          ))}
        </div>
      </div>

      {/* ── Footer ── */}
      <div style={{ marginTop: 20, display: 'flex', justifyContent: 'space-between', color: '#333', fontSize: 10, letterSpacing: '0.12em' }}>
        <span>SCOPE: {runnerSets[0]?.scope.toUpperCase()}</span>
        <span>AUTO-REFRESH 1S<span className="blink">_</span></span>
      </div>
    </div>
  )
}
