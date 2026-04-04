import { colors } from '../theme'

interface Props {
  message: string
  onRetry?: () => void
  onDismiss?: () => void
}

export function ErrorBanner({ message, onRetry, onDismiss }: Props) {
  if (!message) return null

  return (
    <div style={{
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'space-between',
      padding: '10px 14px',
      margin: '8px 12px',
      background: `${colors.accentCoral}26`,
      border: `1px solid ${colors.accentCoral}4D`,
      borderRadius: '8px',
      color: '#fca5a5',
      fontSize: '13px',
    }}>
      <span style={{ flex: 1 }}>{message}</span>
      {onRetry && (
        <button
          onClick={onRetry}
          style={{
            background: 'none',
            border: `1px solid ${colors.accentCoral}4D`,
            color: '#fca5a5',
            cursor: 'pointer',
            padding: '4px 10px',
            borderRadius: '6px',
            fontSize: '12px',
            marginLeft: '8px',
          }}
        >
          Retry
        </button>
      )}
      {onDismiss && (
        <button
          onClick={onDismiss}
          style={{
            background: 'none',
            border: 'none',
            color: '#fca5a5',
            cursor: 'pointer',
            padding: '0 0 0 8px',
            fontSize: '14px',
          }}
        >
          ✕
        </button>
      )}
    </div>
  )
}
