import { useCallback, useEffect, useState } from 'react'
import { checkBackend, saveConfig, testGitHubAuth, validateConfig } from '../api/fetchers'
import { useDashboardStore } from '../store/useDashboardStore'
import type { BackendCheck, Check, ConfigValidationIssue, GitHubAuthResult } from '../types'
import { buildConfigYAML, maskToken } from '../wizard/buildConfigYAML'
import { emptyForm, toAuthInput, toConfig, toRunnerSetForm } from '../wizard/form'
import type { RunnerSetForm, WizardForm } from '../wizard/form'
import { runnerSetPresets } from '../wizard/presets'
import { CheckRows } from './CheckList'
import { RestartPanel } from './RestartPanel'
import { SetupChecklist } from './SetupChecklist'

const stepTitles = ['Target', 'Credentials', 'Runner sets', 'Test', 'Preview', 'Save']

export function SetupWizard({ csrfToken }: { csrfToken: string }) {
  const [step, setStep] = useState(1)
  const [form, setForm] = useState<WizardForm>(emptyForm)

  const update = useCallback((changes: Partial<WizardForm>) => {
    setForm(current => ({ ...current, ...changes }))
  }, [])

  const targetReady = form.targetName.trim() !== '' && (form.targetKind === 'org' || form.targetName.includes('/'))
  const authReady = form.authKind === 'pat'
    ? form.token.trim() !== ''
    : form.clientId.trim() !== '' && /^\d+$/.test(form.installationId.trim()) && form.privateKeyPath.trim() !== ''
  const runnerSetsReady = form.runnerSets.some(set => set.selected)
  const yaml = buildConfigYAML(toConfig(form))

  return (
    <>
      <div className="page-header"><div><h1>Setup</h1><p>Connect GitHub, pick runner sets, and write the config file.</p></div></div>
      <ol className="wizard-steps">
        {stepTitles.map((title, index) => (
          <li className={index + 1 === step ? 'active' : index + 1 < step ? 'done' : ''} key={title}>{index + 1}. {title}</li>
        ))}
      </ol>
      {step === 1 && (
        <>
          <SetupChecklist />
          <TargetStep form={form} update={update} />
          <StepActions step={step} canContinue={targetReady} onBack={() => setStep(step - 1)} onNext={() => setStep(step + 1)} />
        </>
      )}
      {step === 2 && (
        <>
          <CredentialsStep form={form} update={update} />
          <StepActions step={step} canContinue={authReady} onBack={() => setStep(step - 1)} onNext={() => setStep(step + 1)} />
        </>
      )}
      {step === 3 && (
        <>
          <RunnerSetsStep form={form} update={update} />
          <StepActions step={step} canContinue={runnerSetsReady} onBack={() => setStep(step - 1)} onNext={() => setStep(step + 1)} />
        </>
      )}
      {step === 4 && (
        <TestStep form={form} onBack={() => setStep(step - 1)} onNext={() => setStep(step + 1)} />
      )}
      {step === 5 && (
        <PreviewStep yaml={yaml} token={form.authKind === 'pat' ? form.token : ''} onBack={() => setStep(step - 1)} onNext={() => setStep(step + 1)} />
      )}
      {step === 6 && (
        <SaveStep yaml={yaml} csrfToken={csrfToken} onBack={() => setStep(step - 1)} />
      )}
    </>
  )
}

function StepActions({ step, canContinue, onBack, onNext, nextLabel = 'Next' }: {
  step: number
  canContinue: boolean
  onBack: () => void
  onNext: () => void
  nextLabel?: string
}) {
  return (
    <div className="wizard-actions">
      <button className="text-button" disabled={step === 1} onClick={onBack}>Back</button>
      <button className="primary-button" disabled={!canContinue} onClick={onNext}>{nextLabel}</button>
    </div>
  )
}

function TargetStep({ form, update }: { form: WizardForm; update: (changes: Partial<WizardForm>) => void }) {
  return (
    <section className="panel">
      <div className="panel-header"><h2>Where do the runners register?</h2></div>
      <div className="radio-row">
        <label><input type="radio" name="targetKind" checked={form.targetKind === 'org'} onChange={() => update({ targetKind: 'org' })} /> Organization</label>
        <label><input type="radio" name="targetKind" checked={form.targetKind === 'repo'} onChange={() => update({ targetKind: 'repo' })} /> Repository</label>
      </div>
      <label>
        {form.targetKind === 'org' ? 'Organization name' : 'Repository as owner/name'}
        <input
          placeholder={form.targetKind === 'org' ? 'my-org' : 'my-org/my-repo'}
          value={form.targetName}
          onChange={event => update({ targetName: event.target.value })}
        />
      </label>
    </section>
  )
}

function CredentialsStep({ form, update }: { form: WizardForm; update: (changes: Partial<WizardForm>) => void }) {
  const isOrg = form.targetKind === 'org'
  return (
    <section className="panel">
      <div className="panel-header"><h2>How does the daemon talk to GitHub?</h2></div>
      <div className="radio-row">
        <label><input type="radio" name="authKind" checked={form.authKind === 'pat'} onChange={() => update({ authKind: 'pat' })} /> Personal access token</label>
        <label><input type="radio" name="authKind" checked={form.authKind === 'app'} onChange={() => update({ authKind: 'app' })} /> GitHub App</label>
      </div>
      <p className="help-text">
        {form.authKind === 'pat'
          ? isOrg
            ? 'A classic token needs the admin:org scope. A fine grained token needs the organization permission "Self-hosted runners: Read and write".'
            : 'A classic token needs the repo scope. A fine grained token needs the repository permission "Administration: Read and write".'
          : isOrg
            ? 'The app needs the organization permission "Self-hosted runners: Read and write" and must be installed on the organization.'
            : 'The app needs the repository permission "Administration: Read and write" and must be installed on the repository.'}
      </p>
      {form.authKind === 'pat'
        ? (
          <label>
            Token
            <input autoComplete="off" type="password" value={form.token} onChange={event => update({ token: event.target.value })} />
          </label>
        )
        : (
          <>
            <label>
              Client ID
              <input placeholder="Iv1.xxxxxxxxxxxxxxxx" value={form.clientId} onChange={event => update({ clientId: event.target.value })} />
            </label>
            <label>
              Installation ID
              <input inputMode="numeric" placeholder="12345678" value={form.installationId} onChange={event => update({ installationId: event.target.value })} />
            </label>
            <label>
              Private key path on the daemon host
              <input placeholder="/path/to/private-key.pem" value={form.privateKeyPath} onChange={event => update({ privateKeyPath: event.target.value })} />
            </label>
          </>
        )}
      {isOrg && (
        <label>
          Runner group
          <input value={form.runnerGroup} onChange={event => update({ runnerGroup: event.target.value })} />
        </label>
      )}
    </section>
  )
}

function RunnerSetsStep({ form, update }: { form: WizardForm; update: (changes: Partial<WizardForm>) => void }) {
  const [loading, setLoading] = useState(form.runnerSets.length === 0)
  const [error, setError] = useState('')
  const hasSets = form.runnerSets.length > 0

  useEffect(() => {
    if (hasSets) return
    let stopped = false
    Promise.all([checkBackend('docker'), checkBackend('tart')])
      .then(([docker, tart]) => {
        if (stopped) return
        const presets = runnerSetPresets(docker, tart)
        update({ runnerSets: presets.map(toRunnerSetForm) })
      })
      .catch(loadError => {
        if (!stopped) setError(String(loadError))
      })
      .finally(() => {
        if (!stopped) setLoading(false)
      })
    return () => {
      stopped = true
    }
  }, [hasSets, update])

  function updateSet(index: number, changes: Partial<RunnerSetForm>) {
    update({ runnerSets: form.runnerSets.map((set, i) => i === index ? { ...set, ...changes } : set) })
  }

  return (
    <section className="panel">
      <div className="panel-header"><h2>Which runner sets should exist?</h2><span>Presets for this host. Edit any field.</span></div>
      {error && <div className="notice danger"><strong>Backend check</strong><span>{error}</span></div>}
      {loading && <div className="empty-state"><strong>Checking docker and tart…</strong></div>}
      {form.runnerSets.map((set, index) => (
        <div className={set.available ? 'preset-card' : 'preset-card disabled'} key={index}>
          <label className="preset-title">
            <input type="checkbox" checked={set.selected} disabled={!set.available} onChange={event => updateSet(index, { selected: event.target.checked })} />
            {' '}{set.backend} runner set
          </label>
          {!set.available && <div className="notice warning"><strong>Unavailable</strong><span>{set.unavailableReason}</span></div>}
          <div className="preset-fields">
            <label>Name<input disabled={!set.available} value={set.name} onChange={event => updateSet(index, { name: event.target.value })} /></label>
            <label>Image<input disabled={!set.available} value={set.image} onChange={event => updateSet(index, { image: event.target.value })} /></label>
            <label>Labels, comma separated<input disabled={!set.available} value={set.labelsText} onChange={event => updateSet(index, { labelsText: event.target.value })} /></label>
            <label>Max runners<input disabled={!set.available} inputMode="numeric" value={set.maxRunners} onChange={event => updateSet(index, { maxRunners: Number(event.target.value) })} /></label>
            <label>Platform, optional<input disabled={!set.available} placeholder="linux/arm64" value={set.platform} onChange={event => updateSet(index, { platform: event.target.value })} /></label>
          </div>
        </div>
      ))}
    </section>
  )
}

function TestStep({ form, onBack, onNext }: { form: WizardForm; onBack: () => void; onNext: () => void }) {
  const [running, setRunning] = useState(true)
  const [auth, setAuth] = useState<GitHubAuthResult | null>(null)
  const [backends, setBackends] = useState<BackendCheck[]>([])
  const [error, setError] = useState('')

  // The form cannot change while this step is open, so the tests run once.
  useEffect(() => {
    let stopped = false
    const backendNames = Array.from(new Set(form.runnerSets.filter(set => set.selected).map(set => set.backend)))
    Promise.all([
      testGitHubAuth(toAuthInput(form)),
      Promise.all(backendNames.map(name => checkBackend(name))),
    ])
      .then(([authResult, backendResults]) => {
        if (stopped) return
        setAuth(authResult)
        setBackends(backendResults)
      })
      .catch(testError => {
        if (!stopped) setError(String(testError))
      })
      .finally(() => {
        if (!stopped) setRunning(false)
      })
    return () => {
      stopped = true
    }
  }, [form])

  const backendChecks: Check[] = backends.map(backend => ({
    name: backend.backend,
    status: backend.available ? 'pass' : 'fail',
    message: backend.available ? `${backend.version} on ${backend.hostOS}/${backend.hostArch}` : backend.error,
  }))
  const allPass = !running && !error && (auth?.ok ?? false) && backends.every(backend => backend.available)

  return (
    <>
      <section className="panel">
        <div className="panel-header"><h2>GitHub access</h2><span>{auth?.target ?? ''}</span></div>
        {running && <div className="empty-state"><strong>Testing…</strong></div>}
        {error && <div className="notice danger"><strong>Test failed</strong><span>{error}</span></div>}
        {auth && <CheckRows checks={auth.checks} />}
      </section>
      <section className="panel">
        <div className="panel-header"><h2>Backends</h2></div>
        {backendChecks.length > 0 && <CheckRows checks={backendChecks} />}
      </section>
      <div className="wizard-actions">
        <button className="text-button" onClick={onBack}>Back</button>
        {!allPass && !running && <button className="text-button" onClick={onNext}>Continue anyway</button>}
        <button className="primary-button" disabled={!allPass} onClick={onNext}>Next</button>
      </div>
    </>
  )
}

function PreviewStep({ yaml, token, onBack, onNext }: { yaml: string; token: string; onBack: () => void; onNext: () => void }) {
  const setDraftYAML = useDashboardStore(state => state.setDraftYAML)

  function openEditor() {
    setDraftYAML(yaml)
    window.location.hash = '#/config'
  }

  return (
    <>
      <section className="panel">
        <div className="panel-header"><h2>Config preview</h2><span>Token hidden</span></div>
        <pre>{maskToken(yaml, token)}</pre>
        <div className="editor-actions">
          <button className="text-button" onClick={openEditor}>Open in advanced editor</button>
        </div>
      </section>
      <div className="wizard-actions">
        <button className="text-button" onClick={onBack}>Back</button>
        <button className="primary-button" onClick={onNext}>Next</button>
      </div>
    </>
  )
}

function SaveStep({ yaml, csrfToken, onBack }: { yaml: string; csrfToken: string; onBack: () => void }) {
  const configStatus = useDashboardStore(state => state.configStatus)
  const [issues, setIssues] = useState<Array<ConfigValidationIssue & { tone: 'danger' | 'warning' }>>([])
  const [checking, setChecking] = useState(true)
  const [saving, setSaving] = useState(false)
  const [saved, setSaved] = useState(false)
  const hasErrors = issues.some(issue => issue.tone === 'danger')

  useEffect(() => {
    let stopped = false
    validateConfig(yaml)
      .then(result => {
        if (stopped) return
        setIssues([
          ...result.errors.map(issue => ({ ...issue, tone: 'danger' as const })),
          ...result.warnings.map(issue => ({ ...issue, tone: 'warning' as const })),
        ])
      })
      .catch(validateError => {
        if (!stopped) setIssues([{ path: '$', message: String(validateError), tone: 'danger' }])
      })
      .finally(() => {
        if (!stopped) setChecking(false)
      })
    return () => {
      stopped = true
    }
  }, [yaml])

  async function save() {
    setSaving(true)
    try {
      const result = await saveConfig(yaml, csrfToken, true)
      if (result.errors.length > 0) {
        setIssues(result.errors.map(issue => ({ ...issue, tone: 'danger' as const })))
        return
      }
      setIssues(result.warnings.map(issue => ({ ...issue, tone: 'warning' as const })))
      setSaved(true)
    } catch (saveError) {
      setIssues([{ path: '$', message: String(saveError), tone: 'danger' }])
    } finally {
      setSaving(false)
    }
  }

  return (
    <>
      {issues.map((issue, index) => (
        <div className={`notice ${issue.tone}`} key={`${issue.path}-${index}`}>
          <strong>{issue.path}</strong>
          <span>{issue.message}</span>
        </div>
      ))}
      {saved
        ? (
          <>
            <section className="panel">
              <div className="panel-header"><h2>Config saved to {configStatus?.path || 'the config file'}</h2></div>
              <div className="notice warning">
                <strong>Restart needed</strong>
                <span>The daemon is still running without this config. Restart it to start the runner sets.</span>
              </div>
              <div className="editor-actions">
                <a className="external-link" href="#/overview">Go to overview</a>
              </div>
            </section>
            <RestartPanel csrfToken={csrfToken} restartCommands={configStatus?.restartCommands ?? []} />
          </>
        )
        : (
          <section className="panel">
            <div className="panel-header"><h2>Write the config file</h2><span>{configStatus?.path || ''}</span></div>
            {checking && <div className="empty-state"><strong>Validating…</strong></div>}
            {!checking && !hasErrors && <div className="notice success"><strong>Valid</strong><span>The config passes validation and can be saved.</span></div>}
          </section>
        )}
      <div className="wizard-actions">
        <button className="text-button" disabled={saving} onClick={onBack}>Back</button>
        {!saved && <button className="primary-button" disabled={checking || hasErrors || saving} onClick={save}>{saving ? 'Saving…' : 'Save config'}</button>}
      </div>
    </>
  )
}
