import { useState } from 'preact/hooks'
import { BottomSheet } from './BottomSheet'
import s from './ConfirmationDrawer.module.css'

interface ConfirmationDrawerProps {
  open: boolean
  onClose: () => void
  onConfirm: () => void | Promise<void>
  title: string
  description?: string
  confirmText?: string
  cancelText?: string
  isDangerous?: boolean
}

export function ConfirmationDrawer({
  open,
  onClose,
  onConfirm,
  title,
  description,
  confirmText = 'Confirm',
  cancelText = 'Cancel',
  isDangerous = false
}: ConfirmationDrawerProps) {
  const [pending, setPending] = useState(false)

  const handleConfirm = async () => {
    if (pending) return
    setPending(true)
    try {
      await onConfirm()
      onClose()
    } catch (err) {
      // Keep drawer open so user can retry or cancel explicitly
      console.error('Confirm action failed:', err)
    } finally {
      setPending(false)
    }
  }

  return (
    <BottomSheet open={open} onClose={onClose} ariaLabel={title}>
      <div className={s.content}>
        <div className={s.title}>{title}</div>
        {description && <div className={s.description}>{description}</div>}
        <div className={s.actions}>
          <button className={s.cancelBtn} onClick={onClose} disabled={pending}>
            {cancelText}
          </button>
          <button
            className={isDangerous ? s.confirmBtnDangerous : s.confirmBtn}
            onClick={handleConfirm}
            disabled={pending}
          >
            {pending ? 'Working…' : confirmText}
          </button>
        </div>
      </div>
    </BottomSheet>
  )
}
