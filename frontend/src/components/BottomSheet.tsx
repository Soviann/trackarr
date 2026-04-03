import type { ComponentChildren } from 'preact'
import { colors } from '../theme'

interface BottomSheetProps {
  open: boolean
  onClose: () => void
  children: ComponentChildren
}

export function BottomSheet({ open, onClose, children }: BottomSheetProps) {
  if (!open) return null

  return (
    <div
      onClick={onClose}
      style={{
        position: 'fixed',
        inset: 0,
        background: 'rgba(0,0,0,0.4)',
        zIndex: 200,
        display: 'flex',
        alignItems: 'flex-end',
      }}
    >
      <div
        onClick={(e: Event) => e.stopPropagation()}
        style={{
          width: '100%',
          background: colors.bgCard,
          borderRadius: '16px 16px 0 0',
          maxHeight: '85vh',
          overflowY: 'auto',
        }}
      >
        {/* Drag handle */}
        <div style={{ display: 'flex', justifyContent: 'center', padding: '10px 0 6px' }}>
          <div style={{ width: '32px', height: '4px', background: 'rgba(255,255,255,0.1)', borderRadius: '2px' }} />
        </div>
        {children}
      </div>
    </div>
  )
}
