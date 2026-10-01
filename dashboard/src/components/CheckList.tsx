import type { Check } from '../types'

export function CheckRows({ checks }: { checks: Check[] }) {
  return (
    <div className="check-list">
      {checks.map(check => (
        <div className="check-row" key={check.name}>
          <span className={`status-badge ${check.status}`}>{check.status}</span>
          <span className="check-name">{check.name}</span>
          <span className="check-message">{check.message}</span>
        </div>
      ))}
    </div>
  )
}
