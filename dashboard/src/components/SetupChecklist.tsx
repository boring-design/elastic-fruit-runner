import { useState } from 'react'
import useSWR from 'swr'
import { fetchSetupChecklist } from '../api/fetchers'

export function SetupChecklist() {
  const checklist = useSWR('setupChecklist', () => fetchSetupChecklist(false), { refreshInterval: 10000 })
  const [checking, setChecking] = useState(false)

  async function recheck() {
    setChecking(true)
    try {
      await checklist.mutate(() => fetchSetupChecklist(true))
    } finally {
      setChecking(false)
    }
  }

  const steps = checklist.data?.steps ?? []
  if (checklist.error) {
    return <div className="notice danger"><strong>Setup checklist</strong><span>{String(checklist.error)}</span></div>
  }
  if (steps.length === 0) return null
  if (steps.every(step => step.status === 'pass')) {
    return <div className="setup-complete">Setup complete</div>
  }
  return (
    <section className="panel">
      <div className="panel-header">
        <h2>Setup checklist</h2>
        <button className="text-button" disabled={checking} onClick={recheck}>{checking ? 'Checking…' : 'Re-check'}</button>
      </div>
      <div className="check-list">
        {steps.map((step, index) => (
          <div className="check-row" key={`${step.id}-${index}`}>
            <span className={`status-badge ${step.status}`}>{step.status}</span>
            <span className="check-name">{step.title}</span>
            <span className="check-message">
              {step.message}
              {step.page && step.status !== 'pass' && <> <a href={`#/${step.page}`}>Open {step.page}</a></>}
            </span>
          </div>
        ))}
      </div>
    </section>
  )
}
