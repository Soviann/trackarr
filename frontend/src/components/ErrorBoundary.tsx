import { Component } from 'preact'
import type { ComponentChildren } from 'preact'
import { colors } from '../theme'

interface Props {
  children: ComponentChildren
}

interface State {
  hasError: boolean
}

export class ErrorBoundary extends Component<Props, State> {
  state = { hasError: false }

  static getDerivedStateFromError(): State {
    return { hasError: true }
  }

  handleReload = () => {
    window.location.reload()
  }

  render() {
    if (!this.state.hasError) return this.props.children

    return (
      <div style={{
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        justifyContent: 'center',
        minHeight: '100vh',
        padding: '32px',
        textAlign: 'center',
        color: colors.textSecondary,
      }}>
        <div style={{ fontSize: '32px', marginBottom: '16px' }}>:(</div>
        <div style={{ fontSize: '15px', fontWeight: 600, color: colors.textPrimary, marginBottom: '8px' }}>
          Something went wrong
        </div>
        <div style={{ fontSize: '13px', marginBottom: '24px' }}>
          An unexpected error occurred. Please reload the page.
        </div>
        <button
          onClick={this.handleReload}
          style={{
            padding: '10px 24px',
            borderRadius: '8px',
            border: `1px solid ${colors.accentTeal}4D`,
            background: `${colors.accentTeal}1A`,
            color: colors.accentTeal,
            fontSize: '14px',
            fontWeight: 600,
            cursor: 'pointer',
          }}
        >
          Reload
        </button>
      </div>
    )
  }
}
