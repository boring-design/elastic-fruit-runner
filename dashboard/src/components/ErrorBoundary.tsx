import { Component, type ErrorInfo, type ReactNode } from 'react'

interface ErrorBoundaryProps {
  children: ReactNode
}

interface ErrorBoundaryState {
  error: Error | null
  componentStack: string | null
}

// ErrorBoundary catches rendering errors in the React tree below it and shows
// a readable fallback UI instead of crashing the whole dashboard to a blank
// screen. Without this guard, a single malformed API record (for example an
// orphan job missing runnerName) crashes the entire app — see issue #69.
export class ErrorBoundary extends Component<ErrorBoundaryProps, ErrorBoundaryState> {
  state: ErrorBoundaryState = { error: null, componentStack: null }

  static getDerivedStateFromError(error: Error): ErrorBoundaryState {
    return { error, componentStack: null }
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    this.setState({ componentStack: info.componentStack ?? null })
    console.error('ErrorBoundary caught a render error', error, info)
  }

  private handleReload = () => {
    window.location.reload()
  }

  render() {
    if (!this.state.error) {
      return this.props.children
    }

    const { error, componentStack } = this.state
    return (
      <div
        className="app-container"
        style={{
          display: 'flex',
          flexDirection: 'column',
          gap: 14,
          alignItems: 'center',
          justifyContent: 'center',
          minHeight: '100vh',
          padding: 24,
          textAlign: 'center',
        }}
      >
        <span style={{ fontSize: 13, fontWeight: 700, letterSpacing: '0.06em', color: '#ff3b30' }}>
          DASHBOARD CRASHED
        </span>
        <span style={{ fontSize: 11, color: '#888', maxWidth: 600, lineHeight: 1.5 }}>
          {error.message || String(error)}
        </span>
        {componentStack ? (
          <details style={{ maxWidth: 720, width: '100%', textAlign: 'left' }}>
            <summary style={{ fontSize: 10, color: '#555', letterSpacing: '0.12em', cursor: 'pointer' }}>
              COMPONENT STACK
            </summary>
            <pre
              style={{
                marginTop: 8,
                padding: 12,
                background: '#111',
                color: '#888',
                fontSize: 10,
                lineHeight: 1.5,
                overflow: 'auto',
                maxHeight: 240,
              }}
            >
              {componentStack}
            </pre>
          </details>
        ) : null}
        <button
          type="button"
          onClick={this.handleReload}
          style={{
            marginTop: 8,
            padding: '8px 16px',
            background: '#1a1a1a',
            color: '#f0f0f0',
            border: '1px solid #333',
            borderRadius: 2,
            fontSize: 11,
            letterSpacing: '0.12em',
            cursor: 'pointer',
            fontFamily: 'inherit',
          }}
        >
          RELOAD DASHBOARD
        </button>
      </div>
    )
  }
}
