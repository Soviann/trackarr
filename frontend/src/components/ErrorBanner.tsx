import s from './ErrorBanner.module.css'

interface Props {
  message: string
  onRetry?: () => void
  onDismiss?: () => void
}

export function ErrorBanner({ message, onRetry, onDismiss }: Props) {
  if (!message) return null

  return (
    <div className={s.banner}>
      <span className={s.message}>{message}</span>
      {onRetry && (
        <button onClick={onRetry} className={s.retryButton}>
          Retry
        </button>
      )}
      {onDismiss && (
        <button onClick={onDismiss} className={s.dismissButton}>
          ✕
        </button>
      )}
    </div>
  )
}
