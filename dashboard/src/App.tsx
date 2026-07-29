import { useEffect, useState } from 'react'
import type { FormEvent } from 'react'
import useSWR from 'swr'
import { fetchSession, login, logout, setupAdmin } from './api/fetchers'
import {
  fetchHostResources,
  fetchJobLogs,
  fetchJobResources,
  fetchJobs,
} from './api/fetchers'
import { useDashboardSync } from './hooks/useDashboardSync'
import { useDashboardStore } from './store/useDashboardStore'
import type {
  ConfigStatus,
  JobRecord,
  MachineVitals,
  RunnerSet,
  ResourceSample,
  SessionState,
} from './types'

type Page = 'overview' | 'jobs' | 'runner-sets' | 'config' | 'system'

const pages: Array<{ id: Page; label: string }> = [
  { id: 'overview', label: 'Overview' },
  { id: 'jobs', label: 'Jobs' },
  { id: 'runner-sets', label: 'Runner Sets' },
  { id: 'config', label: 'Config' },
  { id: 'system', label: 'System' },
]

export default function App() {
  const session = useSWR('session', fetchSession)

  if (session.isLoading) return <FullPageMessage title="Loading console" />
  if (session.error) return <FullPageMessage title="Console unavailable" detail={String(session.error)} />
  if (!session.data) return <FullPageMessage title="Session unavailable" />

  if (!session.data.authenticated) {
    return (
      <AuthScreen
        setupRequired={session.data.setupRequired}
        onComplete={() => session.mutate()}
      />
    )
  }

  return <Console session={session.data} onLogout={() => session.mutate()} />
}

function AuthScreen({
  setupRequired,
  onComplete,
}: {
  setupRequired: boolean
  onComplete: () => Promise<SessionState | undefined>
}) {
  const [setupCode, setSetupCode] = useState('')
  const [password, setPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [error, setError] = useState('')
  const [submitting, setSubmitting] = useState(false)

  async function submit(event: FormEvent) {
    event.preventDefault()
    setError('')
    if (setupRequired && password !== confirmPassword) {
      setError('Passwords do not match.')
      return
    }
    setSubmitting(true)
    try {
      if (setupRequired) {
        await setupAdmin(setupCode, password)
      } else {
        await login(password)
      }
      await onComplete()
    } catch (submitError) {
      setError(submitError instanceof Error ? submitError.message : String(submitError))
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="auth-shell">
      <form className="auth-card" onSubmit={submit}>
        <div className="brand-mark">EFR</div>
        <h1>{setupRequired ? 'Set up console' : 'Sign in'}</h1>
        <p>
          {setupRequired
            ? 'Enter the setup code from the daemon log and create the admin password.'
            : 'Enter the console admin password.'}
        </p>
        {setupRequired && (
          <label>
            Setup code
            <input
              autoComplete="one-time-code"
              value={setupCode}
              onChange={event => setSetupCode(event.target.value)}
              required
            />
          </label>
        )}
        <label>
          Password
          <input
            autoComplete={setupRequired ? 'new-password' : 'current-password'}
            type="password"
            minLength={setupRequired ? 12 : undefined}
            value={password}
            onChange={event => setPassword(event.target.value)}
            required
          />
        </label>
        {setupRequired && (
          <label>
            Confirm password
            <input
              autoComplete="new-password"
              type="password"
              minLength={12}
              value={confirmPassword}
              onChange={event => setConfirmPassword(event.target.value)}
              required
            />
          </label>
        )}
        {error && <div className="notice danger">{error}</div>}
        <button className="primary-button" disabled={submitting} type="submit">
          {submitting ? 'Working…' : setupRequired ? 'Create admin' : 'Sign in'}
        </button>
      </form>
    </div>
  )
}

function Console({ session, onLogout }: { session: SessionState; onLogout: () => Promise<SessionState | undefined> }) {
  const sync = useDashboardSync()
  const store = useDashboardStore()
  const [page, setPage] = useState<Page>(() => pageFromHash())
  const version = getVersion(store.daemonStatus?.buildInfo?.main?.version)

  useEffect(() => {
    const updatePage = () => setPage(pageFromHash())
    window.addEventListener('hashchange', updatePage)
    return () => window.removeEventListener('hashchange', updatePage)
  }, [])

  async function signOut() {
    await logout(session.csrfToken)
    await onLogout()
  }

  if (sync.isLoading) return <FullPageMessage title="Loading operations data" />
  if (sync.error) return <FullPageMessage title="Operations data unavailable" detail={String(sync.error)} />

  return (
    <div className="console-shell">
      <header className="topbar">
        <div>
          <div className="product-name">Elastic Fruit Runner</div>
          <div className="product-meta">{version}</div>
        </div>
        <div className="topbar-actions">
          <ConfigBadge status={store.configStatus} />
          <button className="text-button" onClick={signOut}>Sign out</button>
        </div>
      </header>
      <div className="console-body">
        <nav className="sidebar" aria-label="Console pages">
          {pages.map(item => (
            <a
              className={page === item.id ? 'nav-link active' : 'nav-link'}
              href={`#/${item.id}`}
              key={item.id}
            >
              {item.label}
            </a>
          ))}
          <div className="sidebar-status">
            <StatusDot ok={store.summary?.githubConnected ?? false} />
            GitHub {store.summary?.githubConnected ? 'connected' : 'disconnected'}
          </div>
        </nav>
        <main className="content">
          {page === 'overview' && <Overview />}
          {page === 'jobs' && <JobsPage jobs={store.recentJobs} now={store.now} />}
          {page === 'runner-sets' && <RunnerSetsPage runnerSets={store.runnerSets} />}
          {page === 'config' && store.configStatus && <ConfigPage status={store.configStatus} />}
          {page === 'system' && <SystemPage />}
        </main>
      </div>
    </div>
  )
}

function Overview() {
  const store = useDashboardStore()
  const summary = store.summary
  if (!summary) return <EmptyState title="Summary unavailable" />

  const activeRunners = summary.preparingRunnerCount + summary.idleRunnerCount + summary.busyRunnerCount
  return (
    <>
      <PageHeader title="Overview" detail="Current daemon, runner, job, and host state." />
      <section className="stat-grid">
        <Stat label="Runner sets" value={summary.runnerSetCount} />
        <Stat label="Active runners" value={activeRunners} />
        <Stat label="Busy" value={summary.busyRunnerCount} />
        <Stat label="Running jobs" value={summary.runningJobCount} />
        <Stat label="Failed jobs" value={summary.failedJobCount} tone={summary.failedJobCount ? 'danger' : 'normal'} />
        <Stat label="Completed jobs" value={summary.completedJobCount} />
      </section>
      <div className="two-column">
        <section className="panel">
          <PanelHeader title="Host resources" detail="Current sample" />
          <Vitals vitals={store.machineVitals} />
        </section>
        <section className="panel">
          <PanelHeader title="Service" detail={store.daemonStatus ? `Up ${formatUptime(store.daemonStatus.startedAt, store.now)}` : ''} />
          <DefinitionList
            rows={[
              ['GitHub', summary.githubConnected ? 'Connected' : 'Disconnected'],
              ['Preparing', String(summary.preparingRunnerCount)],
              ['Idle', String(summary.idleRunnerCount)],
              ['Config', configStateLabel(store.configStatus?.state)],
            ]}
          />
        </section>
      </div>
      <section className="panel">
        <PanelHeader title="Recent jobs" detail={`${store.recentJobs.length} records`} />
        <JobTable jobs={store.recentJobs.slice(0, 10)} now={store.now} />
      </section>
    </>
  )
}

function JobsPage({ jobs: initialJobs, now }: { jobs: JobRecord[]; now: Date }) {
  const [status, setStatus] = useState('')
  const [repository, setRepository] = useState('')
  const [workflow, setWorkflow] = useState('')
  const [selected, setSelected] = useState<JobRecord | null>(null)
  const jobs = useSWR(
    ['jobs', status, repository, workflow],
    () => fetchJobs({ status, repository, workflow, pageSize: 100 }),
    { refreshInterval: 5000, fallbackData: { jobs: initialJobs, nextCursor: '' } },
  )
  return (
    <>
      <PageHeader title="Jobs" detail="Recent job history from local storage." />
      <section className="filter-bar">
        <select aria-label="Job status" value={status} onChange={event => setStatus(event.target.value)}>
          <option value="">All results</option>
          <option value="running">Running</option>
          <option value="succeeded">Success</option>
          <option value="failed">Failure</option>
          <option value="canceled">Canceled</option>
        </select>
        <input aria-label="Repository filter" placeholder="Repository" value={repository} onChange={event => setRepository(event.target.value)} />
        <input aria-label="Workflow filter" placeholder="Workflow" value={workflow} onChange={event => setWorkflow(event.target.value)} />
      </section>
      <section className="panel">
        <PanelHeader title="Job records" detail={`${jobs.data?.jobs.length ?? 0} records`} />
        {jobs.error
          ? <EmptyState title="Job history unavailable" detail={String(jobs.error)} />
          : <JobTable jobs={jobs.data?.jobs ?? []} now={now} onSelect={setSelected} />}
      </section>
      {selected && <JobDetail job={selected} onClose={() => setSelected(null)} />}
    </>
  )
}

function JobTable({ jobs, now, onSelect }: { jobs: JobRecord[]; now: Date; onSelect?: (job: JobRecord) => void }) {
  if (jobs.length === 0) return <EmptyState title="No jobs recorded" detail="Jobs appear after a runner accepts work." />
  return (
    <div className="table-wrap">
      <table>
        <thead>
          <tr>
            <th>Job</th>
            <th>Repository</th>
            <th>Runner set</th>
            <th>Runner</th>
            <th>Result</th>
            <th>Started</th>
            <th>Duration</th>
          </tr>
        </thead>
        <tbody>
          {jobs.map(job => (
            <tr className={onSelect ? 'clickable-row' : ''} key={`${job.id}-${job.startedAt.toISOString()}`} onClick={() => onSelect?.(job)}>
              <td className="mono">{job.displayName || job.id}</td>
              <td>{job.owner && job.repository ? `${job.owner}/${job.repository}` : 'Unknown'}</td>
              <td>{job.runnerSetName || 'Unknown'}</td>
              <td>{job.runnerName || 'Unknown'}</td>
              <td><ResultBadge result={job.result} /></td>
              <td>{formatDate(job.startedAt)}</td>
              <td>{formatDuration(job.startedAt, job.completedAt ?? now)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

function JobDetail({ job, onClose }: { job: JobRecord; onClose: () => void }) {
  const logs = useSWR(['jobLogs', job.id], () => fetchJobLogs(job.id), {
    refreshInterval: job.result === 'running' ? 2000 : 0,
  })
  const resources = useSWR(['jobResources', job.id], () => fetchJobResources(job.id), {
    refreshInterval: job.result === 'running' ? 5000 : 0,
  })
  const samples = resources.data ?? []
  const estimate = samples.some(sample => sample.accuracy === 'estimate')
  return (
    <div className="detail-overlay" role="dialog" aria-modal="true" aria-label="Job detail">
      <section className="detail-panel">
        <div className="detail-title">
          <div>
            <span className="eyebrow">Job detail</span>
            <h2>{job.displayName || job.id}</h2>
          </div>
          <button className="text-button" onClick={onClose}>Close</button>
        </div>
        <DefinitionList rows={[
          ['Repository', job.owner && job.repository ? `${job.owner}/${job.repository}` : 'Unknown'],
          ['Workflow', job.workflowRef || 'Unknown'],
          ['Runner', job.runnerName],
          ['Backend', job.backend || 'Unknown'],
          ['Queued', job.queuedAt ? formatDate(job.queuedAt) : 'Unavailable'],
          ['Runner assigned', job.runnerAssignedAt ? formatDate(job.runnerAssignedAt) : 'Unavailable'],
          ['Started', formatDate(job.startedAt)],
          ['Completed', job.completedAt ? formatDate(job.completedAt) : 'Running'],
        ]} />
        {job.actionsURL && <a className="external-link" href={job.actionsURL} target="_blank" rel="noreferrer">Open in GitHub Actions</a>}
        <div className="panel-header"><h2>Resource history</h2><span>{estimate ? 'Tart host estimate' : 'Backend data'}</span></div>
        {estimate && <div className="notice warning">Guest resource usage is unavailable. Tart values show VM allocation and host side estimates.</div>}
        <ResourceChart samples={samples} field="cpuPercent" label="CPU" suffix="%" />
        <ResourceChart samples={samples} field="memoryUsedBytes" label="Memory" format={formatBytes} />
        <div className="panel-header"><h2>Runner log</h2><span>{logs.data?.lines.length ?? 0} chunks</span></div>
        <pre className="job-log">{logs.data?.lines.map(line => line.text).join('') || 'No log data is available.'}</pre>
      </section>
    </div>
  )
}

function RunnerSetsPage({ runnerSets }: { runnerSets: RunnerSet[] }) {
  return (
    <>
      <PageHeader title="Runner Sets" detail="Live capacity and runner state for each GitHub scope." />
      {runnerSets.length === 0 && <section className="panel"><EmptyState title="No runner sets configured" /></section>}
      <div className="card-list">
        {runnerSets.map(runnerSet => (
          <section className="panel" key={runnerSet.name}>
            <PanelHeader
              title={runnerSet.name}
              detail={`${runnerSet.backend.toUpperCase()} · ${runnerSet.runners.length}/${runnerSet.maxRunners}`}
            />
            <DefinitionList
              rows={[
                ['Scope', runnerSet.scope],
                ['Image', runnerSet.image],
                ['Labels', runnerSet.labels.join(', ') || 'None'],
                ['GitHub', runnerSet.connected ? 'Connected' : 'Disconnected'],
              ]}
            />
            <div className="runner-counts">
              {(['preparing', 'idle', 'busy'] as const).map(state => (
                <span key={state}>{state}: {runnerSet.runners.filter(runner => runner.state === state).length}</span>
              ))}
            </div>
            {runnerSet.runners.length > 0 && (
              <div className="table-wrap">
                <table>
                  <thead><tr><th>Runner</th><th>State</th><th>Since</th></tr></thead>
                  <tbody>
                    {runnerSet.runners.map(runner => (
                      <tr key={runner.name}>
                        <td>{runner.name}</td>
                        <td><span className={`status-badge ${runner.state}`}>{runner.state}</span></td>
                        <td>{formatDate(runner.since)}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </section>
        ))}
      </div>
    </>
  )
}

function ConfigPage({ status }: { status: ConfigStatus }) {
  const sameText = status.activeYAML === status.diskYAML
  return (
    <>
      <PageHeader title="Config" detail="Read only view. Disk changes do not affect the running process." />
      <div className={`notice ${status.state === 'disk_invalid' ? 'danger' : status.state === 'restart_required' ? 'warning' : 'success'}`}>
        <strong>{configStateLabel(status.state)}</strong>
        <span>
          {status.state === 'in_sync' && 'Disk config matches the active config.'}
          {status.state === 'restart_required' && 'Disk config changed. Manual service restart is required to apply it.'}
          {status.state === 'disk_invalid' && 'Disk config cannot be read or parsed. The active config is unchanged.'}
        </span>
      </div>
      {status.validationErrors.map(error => <div className="notice danger" key={error}>{error}</div>)}
      <section className="panel">
        <PanelHeader title="Config identity" detail={status.path || 'No config file'} />
        <DefinitionList
          rows={[
            ['Active hash', shortenHash(status.activeHash)],
            ['Disk hash', shortenHash(status.diskHash)],
            ['Active loaded', formatDate(status.activeLoadedAt)],
            ['Disk modified', status.diskModifiedAt ? formatDate(status.diskModifiedAt) : 'Unknown'],
          ]}
        />
      </section>
      <div className="config-grid">
        <section className="panel">
          <PanelHeader title="Active config" detail="Secrets hidden" />
          <pre>{status.activeYAML || 'No active YAML available.'}</pre>
        </section>
        <section className="panel">
          <PanelHeader title="Disk config" detail={sameText ? 'Matches active text' : 'Differs from active text'} />
          <pre>{status.diskYAML || 'No valid disk YAML available.'}</pre>
        </section>
      </div>
    </>
  )
}

function SystemPage() {
  const store = useDashboardStore()
  const system = store.systemInfo
  const daemon = store.daemonStatus
  const history = useSWR(
    'hostResources',
    () => fetchHostResources(new Date(Date.now() - 60 * 60 * 1000), new Date()),
    { refreshInterval: 5000 },
  )
  return (
    <>
      <PageHeader title="System" detail="Daemon build, storage, and current host health." />
      <div className="two-column">
        <section className="panel">
          <PanelHeader title="Runtime" />
          <DefinitionList
            rows={[
              ['Version', getVersion(daemon?.buildInfo?.main?.version)],
              ['Go', system?.goVersion || daemon?.buildInfo?.goVersion || 'Unknown'],
              ['Platform', system ? `${system.os}/${system.arch}` : 'Unknown'],
              ['Started', daemon ? formatDate(daemon.startedAt) : 'Unknown'],
              ['Uptime', daemon ? formatUptime(daemon.startedAt, store.now) : 'Unknown'],
            ]}
          />
        </section>
        <section className="panel">
          <PanelHeader title="Storage" />
          <DefinitionList
            rows={[
              ['Database', system?.databasePath || 'Unknown'],
              ['Database size', formatBytes(system?.databaseSizeBytes ?? 0)],
              ['Config', store.configStatus?.path || 'No config file'],
            ]}
          />
        </section>
      </div>
      <section className="panel">
        <PanelHeader title="Host resources" detail={history.data?.earliestAt ? `Available since ${formatDate(history.data.earliestAt)}` : 'Current sample'} />
        <Vitals vitals={store.machineVitals} />
        <ResourceChart samples={history.data?.samples ?? []} field="cpuPercent" label="CPU history" suffix="%" />
        <ResourceChart samples={history.data?.samples ?? []} field="memoryUsedBytes" label="Memory history" format={formatBytes} />
      </section>
    </>
  )
}

function ResourceChart({
  samples,
  field,
  label,
  suffix = '',
  format,
}: {
  samples: ResourceSample[]
  field: 'cpuPercent' | 'memoryUsedBytes'
  label: string
  suffix?: string
  format?: (value: number) => string
}) {
  if (samples.length < 2) return <EmptyState title={`${label} has no history yet`} />
  const values = samples.map(sample => sample[field])
  const maxValue = Math.max(...values, 1)
  const points = values.map((value, index) => {
    const x = values.length === 1 ? 0 : index / (values.length - 1) * 100
    const y = 34 - value / maxValue * 30
    return `${x},${y}`
  }).join(' ')
  const latest = values.at(-1) ?? 0
  return (
    <div className="resource-chart">
      <div><span>{label}</span><strong>{format ? format(latest) : `${latest.toFixed(1)}${suffix}`}</strong></div>
      <svg viewBox="0 0 100 36" preserveAspectRatio="none" role="img" aria-label={label}>
        <polyline points={points} />
      </svg>
    </div>
  )
}

function Vitals({ vitals }: { vitals: MachineVitals | null }) {
  if (!vitals) return <EmptyState title="No host sample available" />
  const entries = [
    ['CPU', vitals.cpuUsagePercent],
    ['Memory', vitals.memoryUsagePercent],
    ['Disk', vitals.diskUsagePercent],
  ] as const
  return (
    <div className="vitals-grid">
      {entries.map(([label, value]) => (
        <div className="vital" key={label}>
          <div><span>{label}</span><strong>{value.toFixed(1)}%</strong></div>
          <div className="meter"><span style={{ width: `${Math.min(100, value)}%` }} /></div>
        </div>
      ))}
      <div className="vital">
        <div><span>Temperature</span><strong>{vitals.temperatureCelsius ? `${vitals.temperatureCelsius.toFixed(1)}°C` : 'Unavailable'}</strong></div>
      </div>
    </div>
  )
}

function Stat({ label, value, tone = 'normal' }: { label: string; value: number; tone?: 'normal' | 'danger' }) {
  return <div className={`stat ${tone}`}><span>{label}</span><strong>{value}</strong></div>
}

function PageHeader({ title, detail }: { title: string; detail: string }) {
  return <div className="page-header"><div><h1>{title}</h1><p>{detail}</p></div></div>
}

function PanelHeader({ title, detail = '' }: { title: string; detail?: string }) {
  return <div className="panel-header"><h2>{title}</h2>{detail && <span>{detail}</span>}</div>
}

function DefinitionList({ rows }: { rows: Array<[string, string]> }) {
  return <dl>{rows.map(([label, value]) => <div key={label}><dt>{label}</dt><dd>{value}</dd></div>)}</dl>
}

function ResultBadge({ result }: { result: JobRecord['result'] }) {
  return <span className={`status-badge ${result}`}>{result}</span>
}

function ConfigBadge({ status }: { status: ConfigStatus | null }) {
  if (!status) return null
  return <span className={`config-badge ${status.state}`}>{configStateLabel(status.state)}</span>
}

function StatusDot({ ok }: { ok: boolean }) {
  return <span className={ok ? 'status-dot ok' : 'status-dot'} aria-hidden="true" />
}

function EmptyState({ title, detail = '' }: { title: string; detail?: string }) {
  return <div className="empty-state"><strong>{title}</strong>{detail && <span>{detail}</span>}</div>
}

function FullPageMessage({ title, detail = '' }: { title: string; detail?: string }) {
  return <div className="full-page-message"><strong>{title}</strong>{detail && <span>{detail}</span>}</div>
}

function pageFromHash(): Page {
  const value = window.location.hash.replace(/^#\//, '')
  return pages.some(page => page.id === value) ? value as Page : 'overview'
}

function getVersion(version?: string) {
  if (!version || version === '(devel)') return 'dev'
  return version
}

function configStateLabel(state?: ConfigStatus['state']) {
  if (state === 'in_sync') return 'In sync'
  if (state === 'restart_required') return 'Restart required'
  if (state === 'disk_invalid') return 'Disk config invalid'
  return 'Unknown'
}

function shortenHash(value: string) {
  return value ? value.slice(0, 12) : 'Unavailable'
}

function formatDate(value: Date) {
  if (Number.isNaN(value.getTime()) || value.getTime() === 0) return 'Unknown'
  return new Intl.DateTimeFormat(undefined, {
    dateStyle: 'medium',
    timeStyle: 'medium',
  }).format(value)
}

function formatDuration(start: Date, end: Date) {
  const seconds = Math.max(0, Math.floor((end.getTime() - start.getTime()) / 1000))
  if (seconds < 60) return `${seconds}s`
  const minutes = Math.floor(seconds / 60)
  if (minutes < 60) return `${minutes}m ${seconds % 60}s`
  return `${Math.floor(minutes / 60)}h ${minutes % 60}m`
}

function formatUptime(start: Date, now: Date) {
  return formatDuration(start, now)
}

function formatBytes(bytes: number) {
  if (!bytes) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const index = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1)
  return `${(bytes / 1024 ** index).toFixed(index ? 1 : 0)} ${units[index]}`
}
