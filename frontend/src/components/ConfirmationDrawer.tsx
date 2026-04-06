import { BottomSheet } from './BottomSheet'
import s from './ConfirmationDrawer.module.css'

interface ConfirmationDrawerProps {
  open: boolean
  onClose: () => void
  onConfirm: () => void
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
  return (
    <BottomSheet open={open} onClose={onClose}>
      <div className={s.content}>
        <div className={s.title}>{title}</div>
        {description && <div className={s.description}>{description}</div>}
        <div className={s.actions}>
          <button className={s.cancelBtn} onClick={onClose}>
            {cancelText}
          </button>
          <button
            className={isDangerous ? s.confirmBtnDangerous : s.confirmBtn}
            onClick={() => {
              onConfirm()
              onClose()
            }}
          >
            {confirmText}
          </button>
        </div>
      </div>
    </BottomSheet>
  )
}
