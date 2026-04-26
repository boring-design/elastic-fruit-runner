import { useDashboardSync } from './hooks/useDashboardSync'
import { useDashboardDerived } from './hooks/useDashboardDerived'
import { elapsed, fmtDuration, fmtUptime } from './utils'
import { PixelPet } from './components/PixelPet'
import { MOOD_LABEL, MOOD_SUBTEXT } from './components/petMood'
import { SystemVitals } from './components/SystemVitals'
import { RunnerSetPanel } from './components/RunnerSetPanel'
import { JobRow } from './components/JobRow'
import { ConnectionStatus } from './components/ConnectionStatus'

export default function App() {
  const { isLoading, error } = useDashboardSync()
  const {
    daemonStatus,
    runnerSets,
    recentJobs,
    machineVitals,
    now,
    uptime,
    totalMax,
    totalActive,
    preparing,
    idle,
    busy,
    utilPct,
    successCount,
    failureCount,
    canceledCount,
    mood,
  } = useDashboardDerived()

  if (error) {
    return (
      <div className="app-container" style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', minHeight: '100vh', flexDirection: 'column', gap: 12 }}>
        <span style={{ fontSize: 13, fontWeight: 700, letterSpacing: '0.06em', color: '#ff3b30' }}>FAILED TO LOAD</span>
        <span style={{ fontSize: 11, color: '#555' }}>{String(error)}</span>
      </div>
    )
  }

  if (isLoading || !daemonStatus) {
    return (
      <div className="app-container" style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', minHeight: '100vh' }}>
        <div className="spinner" />
      </div>
    )
  }

  const version = daemonStatus.buildInfo?.main?.version && daemonStatus.buildInfo.main.version !== '(devel)'
    ? daemonStatus.buildInfo.main.version
    : 'dev'
  const vcsRevision = daemonStatus.buildInfo?.settings.find(setting => setting.key === 'vcs.revision')?.value ?? 'unknown'

  return (
    <div className="app-container">

      {/* ── Header ── */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 20 }}>
        <div style={{ display: 'flex', alignItems: 'baseline', gap: 20 }}>
          <span style={{ fontSize: 15, fontWeight: 700, letterSpacing: '0.06em' }}>
            ELASTIC-FRUIT-RUNNER
          </span>
          <span style={{ color: '#555', fontSize: 11, letterSpacing: '0.08em' }}>
            v{version} · {vcsRevision}
          </span>
        </div>
        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          <ConnectionStatus connected={daemonStatus.githubConnected} />
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
              <div style={{ fontSize: 16, fontWeight: 700, color: successCount + failureCount + canceledCount > 0 && (failureCount + canceledCount) / (successCount + failureCount + canceledCount) > 0.2 ? '#ff9500' : '#f0f0f0' }}>
                {successCount + failureCount + canceledCount > 0
                  ? `${Math.round(successCount / (successCount + failureCount + canceledCount) * 100)}%`
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
            <SystemVitals vitals={machineVitals} />
          </div>
        </div>
      </div>

      {/* ── Stats row ── */}
      <div className="grid-stats">
        {[
          { label: 'PREPARING', value: preparing,    color: '#aaa' },
          { label: 'IDLE',      value: idle,         color: '#888' },
          { label: 'BUSY',      value: busy,         color: '#f0f0f0' },
          { label: 'SUCCEEDED', value: successCount, color: '#f0f0f0' },
          { label: 'FAILED',    value: failureCount, color: failureCount > 0 ? '#ff3b30' : '#444' },
          { label: 'CANCELED',  value: canceledCount, color: canceledCount > 0 ? '#ff9500' : '#444' },
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
