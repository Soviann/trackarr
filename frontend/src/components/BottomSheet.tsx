import type { ComponentChildren } from 'preact'
import s from './BottomSheet.module.css'

interface BottomSheetProps {
  open: boolean
  onClose: () => void
  children: ComponentChildren
}

export function BottomSheet({ open, onClose, children }: BottomSheetProps) {
  if (!open) return null

  return (
    <div onClick={onClose} className={s.overlay}>
      <div onClick={(e: Event) => e.stopPropagation()} className={s.sheet}>
        {/* Drag handle */}
        <div className={s.handleBar}>
          <div className={s.handle} />
        </div>
        {children}
      </div>
    </div>
  )
}
