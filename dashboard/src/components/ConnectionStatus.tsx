export function ConnectionStatus({ connected }: { connected: boolean | null }) {
  if (connected === null) {
    return (
      <>
        <span className="pulse" style={{ fontSize: 10, color: '#888' }}>●</span>
        <span style={{ fontSize: 11, letterSpacing: '0.12em', color: '#888' }}>
          CHECKING...
        </span>
      </>
    )
  }

  const color = connected ? '#f0f0f0' : '#ff3b30'
  const label = connected ? 'CONNECTED' : 'DISCONNECTED'

  return (
    <>
      <span className="pulse" style={{ fontSize: 10, color }}>●</span>
      <span style={{ fontSize: 11, letterSpacing: '0.12em', color }}>
        {label}
      </span>
    </>
  )
}
