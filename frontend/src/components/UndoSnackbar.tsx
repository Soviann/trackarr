import { useUndo } from '../context/UndoContext'
import { useTranslation } from '../i18n'
import clsx from 'clsx'
import s from './UndoSnackbar.module.css'

interface UndoSnackbarProps {
  hideNavbar?: boolean
}

const RADIUS = 13
const CIRCUMFERENCE = 2 * Math.PI * RADIUS // ~81.68

export function UndoSnackbar({ hideNavbar = false }: UndoSnackbarProps) {
  const { activeUndo, progress, remainingSeconds, triggerUndo, dismiss } = useUndo()
  const { t } = useTranslation()

  if (!activeUndo) {
    return null
  }

  const strokeDashoffset = (1 - progress) * CIRCUMFERENCE

  return (
    <div
      className={clsx(s.snackbarContainer, hideNavbar && s.noNavbar)}
      role="status"
      aria-live="polite"
    >
      {/* Perimeter radial clock timer */}
      <div className={s.timerWrapper} aria-hidden="true">
        <svg className={s.timerSvg} viewBox="0 0 32 32">
          {/* Background track circle */}
          <circle
            cx="16"
            cy="16"
            r={RADIUS}
            fill="none"
            strokeWidth="2.5"
            className={s.timerTrack}
          />
          {/* Animated draining perimeter progress ring */}
          <circle
            cx="16"
            cy="16"
            r={RADIUS}
            fill="none"
            strokeWidth="2.5"
            strokeDasharray={CIRCUMFERENCE}
            strokeDashoffset={strokeDashoffset}
            strokeLinecap="round"
            className={s.timerProgress}
            transform="rotate(-90 16 16)"
          />
        </svg>
        <span className={s.timerSeconds}>{remainingSeconds}</span>
      </div>

      {/* Message */}
      <div className={s.message} title={activeUndo.message}>
        {activeUndo.message}
      </div>

      {/* Actions */}
      <div className={s.actions}>
        <button
          type="button"
          className={s.undoBtn}
          onClick={triggerUndo}
        >
          {activeUndo.actionLabel ?? t('undo.undo')}
        </button>
        <button
          type="button"
          className={s.closeBtn}
          onClick={dismiss}
          aria-label={t('undo.dismiss')}
          title={t('undo.dismiss')}
        >
          <svg
            width="14"
            height="14"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            strokeWidth="2.5"
            strokeLinecap="round"
            strokeLinejoin="round"
          >
            <line x1="18" y1="6" x2="6" y2="18" />
            <line x1="6" y1="6" x2="18" y2="18" />
          </svg>
        </button>
      </div>
    </div>
  )
}
