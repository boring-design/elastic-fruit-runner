import { useDashboardSync } from './hooks/useDashboardSync'
import { useDashboardDerived } from './hooks/useDashboardDerived'
import { elapsed, fmtDuration, fmtUptime } from './utils'
import { PixelPet } from './components/PixelPet'
import { MOOD_LABEL, MOOD_SUBTEXT, MOOD_TOOLTIP } from './components/petMood'
import { SystemVitals } from './components/SystemVitals'
import { RunnerSetPanel } from './components/RunnerSetPanel'
import { JobRow } from './components/JobRow'
import { ConnectionStatus } from './components/ConnectionStatus'

function formatBuildVersion(version: string) {
  if (!version) return 'unknown'
  if (version === 'dev' || version === 'unknown' || version.startsWith('v')) return version
  return `v${version}`
}

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
            {formatBuildVersion(version)} · {vcsRevision}
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
          <div className="status-hero">
            <div className="spinner" />
            <div>
              <div className="status-hero-title">LISTENING FOR JOBS</div>
              <div className="status-hero-sub">GITHUB SCALE SET API</div>
            </div>
          </div>

          <div className="status-meta">
            <div>
              <div className="meta-label">CONNECTED TO</div>
              <div className="meta-value ellipsis" title={runnerSets[0]?.scope ?? ''}>
                {runnerSets[0]?.scope ?? '—'}
              </div>
            </div>

            <div className="status-meta-row">
              <div>
                <div className="meta-label">AUTH MODE</div>
                <div className="meta-value">GITHUB APP</div>
              </div>
              <div>
                <div className="meta-label">RUNNER SETS</div>
                <div className="meta-strong">{runnerSets.length}</div>
              </div>
            </div>

            <div>
              <div className="meta-label">LAST JOB</div>
              <div className="meta-value">
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
          <div className="capacity-sub">
            {utilPct}% UTILIZED · {totalMax - totalActive} SLOTS FREE
          </div>

          {/* Per-set breakdown */}
          <div className="capacity-section">
            <div className="meta-label" style={{ marginBottom: 8 }}>PER SET</div>
            {runnerSets.map(rs => {
              const pct = Math.round((rs.runners.length / rs.maxRunners) * 100)
              return (
                <div key={rs.name} className="capacity-set">
                  <div className="capacity-set-head">
                    <span className="ellipsis" title={rs.name} style={{ fontSize: 10, color: '#888', minWidth: 0 }}>
                      {rs.name}
                    </span>
                    <span style={{ fontSize: 10, color: '#555', flexShrink: 0 }}>
                      {rs.runners.length}/{rs.maxRunners}
                    </span>
                  </div>
                  <div className="capacity-set-bar">
                    <div className="capacity-set-fill" style={{
                      width: `${pct}%`,
                      background: pct >= 80 ? '#ff9500' : '#e8e8e8',
                    }} />
                  </div>
                </div>
              )
            })}
          </div>

          {/* Throughput stats */}
          <div className="capacity-section throughput">
            <div>
              <div className="meta-label">AVG DURATION</div>
              <div className="meta-strong">
                {(() => {
                  const done = recentJobs.filter(j => j.completedAt)
                  if (!done.length) return '—'
                  const avg = done.reduce((s, j) => s + (j.completedAt!.getTime() - j.startedAt.getTime()) / 1000, 0) / done.length
                  return fmtDuration(Math.round(avg))
                })()}
              </div>
            </div>
            <div>
              <div className="meta-label">SUCCESS RATE</div>
              <div className="meta-strong" style={{ color: successCount + failureCount + canceledCount > 0 && (failureCount + canceledCount) / (successCount + failureCount + canceledCount) > 0.2 ? '#ff9500' : '#f0f0f0' }}>
                {successCount + failureCount + canceledCount > 0
                  ? `${Math.round(successCount / (successCount + failureCount + canceledCount) * 100)}%`
                  : '—'}
              </div>
            </div>
          </div>
        </div>

        {/* Cell: CONTROLLER — pixel pet + uptime + vitals */}
        <div className="cell cell-brain">
          <div className="label" title="Live state of the controller process and host machine">
            CONTROLLER
          </div>
          <div className="brain-pet-row">
            {/* Pixel pet */}
            <div className="brain-pet">
              <PixelPet mood={mood} />
            </div>
            {/* Status text + uptime */}
            <div className="brain-info">
              <div
                className="brain-mood"
                title={MOOD_TOOLTIP[mood]}
              >
                {MOOD_LABEL[mood]}
              </div>
              <div className="brain-mood-sub">
                {MOOD_SUBTEXT[mood]}
              </div>
              <div className="brain-stats">
                <div>
                  <div className="brain-stat-label" title="How long the controller process has been running">
                    UPTIME
                  </div>
                  <div className="brain-stat-value">
                    {fmtUptime(uptime)}
                  </div>
                </div>
                <div>
                  <div className="brain-stat-label" title="Idle runners are removed after this duration">
                    IDLE TIMEOUT
                  </div>
                  <div className="brain-stat-value">
                    {Math.floor(daemonStatus.idleTimeout / 60)}m
                  </div>
                </div>
              </div>
            </div>
          </div>
          {/* System vitals */}
          <div className="brain-vitals">
            <div className="label" style={{ marginBottom: 10 }}>HOST VITALS</div>
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
          <div className="label" style={{ marginBottom: 14 }}>RUNNER SETS</div>
          {runnerSets.map(rs => (
            <RunnerSetPanel key={rs.name} rs={rs} now={now} />
          ))}
        </div>

        {/* Recent Jobs */}
        <div className="cell">
          <div className="label" style={{ marginBottom: 10 }}>RECENT JOBS</div>
          <div className="job-row job-row-head">
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
