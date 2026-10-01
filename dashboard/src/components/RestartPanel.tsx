import { useEffect, useRef, useState } from 'react'
import { fetchDaemonStatus, restartService } from '../api/fetchers'

const pollInterval = 1000
const restartTimeout = 90000
const sessionLostMessage = 'session is not valid'

type Phase =
  | { kind: 'idle' }
  | { kind: 'confirm'; busyRunnerCount: number }
  | { kind: 'restarting'; startedAt: Date }
  | { kind: 'timeout' }

export function RestartPanel({ csrfToken, restartCommands }: { csrfToken: string; restartCommands: string[] }) {
  const [phase, setPhase] = useState<Phase>({ kind: 'idle' })
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  // Phase is read inside the poll timer, so keep the latest value in a ref.
  const phaseRef = useRef(phase)
  phaseRef.current = phase

  useEffect(() => {
    if (phase.kind !== 'restarting') return
    const previousStart = phase.startedAt.getTime()
    const deadline = Date.now() + restartTimeout
    let stopped = false

    async function poll() {
      if (stopped) return
      try {
        const status = await fetchDaemonStatus()
        if (status.startedAt.getTime() > previousStart) {
          window.location.reload()
          return
        }
      } catch (pollError) {
        // The daemon is down while it restarts. A lost session means the auth database changed.
        if (String(pollError).includes(sessionLostMessage)) {
          window.location.reload()
          return
        }
      }
      if (Date.now() > deadline) {
        setPhase({ kind: 'timeout' })
        return
      }
      window.setTimeout(poll, pollInterval)
    }

    window.setTimeout(poll, pollInterval)
    return () => {
      stopped = true
    }
  }, [phase])

  async function restart(force: boolean) {
    setBusy(true)
    setError('')
    try {
      const before = await fetchDaemonStatus()
      const result = await restartService(force, csrfToken)
      if (!result.accepted) {
        setPhase({ kind: 'confirm', busyRunnerCount: result.busyRunnerCount })
        return
      }
      setPhase({ kind: 'restarting', startedAt: before.startedAt })
    } catch (restartError) {
      setError(String(restartError))
    } finally {
      setBusy(false)
    }
  }

  if (phase.kind === 'restarting') {
    return (
      <section className="panel">
        <div className="panel-header"><h2>Restarting</h2><span>Waiting for the daemon to come back</span></div>
        <div className="empty-state"><strong>Restarting…</strong><span>This page reloads when the daemon is back.</span></div>
      </section>
    )
  }

  return (
    <section className="panel">
      <div className="panel-header"><h2>Apply config</h2><span>A restart can interrupt running jobs.</span></div>
      {error && <div className="notice danger"><strong>Restart failed</strong><span>{error}</span></div>}
      {phase.kind === 'timeout' && (
        <div className="notice warning">
          <strong>The daemon did not come back in time</strong>
          <span>Restart it with the command that matches your install, then reload this page.</span>
        </div>
      )}
      {phase.kind === 'confirm' && (
        <div className="notice warning">
          <strong>{phase.busyRunnerCount} {phase.busyRunnerCount === 1 ? 'job is' : 'jobs are'} running and will be canceled.</strong>
          <span>Restart anyway?</span>
        </div>
      )}
      <div className="editor-actions">
        {phase.kind === 'confirm'
          ? (
            <>
              <button className="text-button" disabled={busy} onClick={() => setPhase({ kind: 'idle' })}>Cancel</button>
              <button className="primary-button" disabled={busy} onClick={() => restart(true)}>{busy ? 'Working…' : 'Restart anyway'}</button>
            </>
          )
          : <button className="primary-button" disabled={busy} onClick={() => restart(false)}>{busy ? 'Working…' : 'Restart to apply'}</button>}
      </div>
      <details className="restart-fallback">
        <summary>Restart from a shell instead</summary>
        {restartCommands.map(command => <pre key={command}>{command}</pre>)}
      </details>
    </section>
  )
}
