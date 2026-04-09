import { useState, useRef } from 'preact/hooks'
import type { ComponentChildren } from 'preact'
import s from './BottomSheet.module.css'

interface BottomSheetProps {
  open: boolean
  onClose: () => void
  children: ComponentChildren
}

export function BottomSheet({ open, onClose, children }: BottomSheetProps) {
  const [dragY, setDragY] = useState(0)
  const touchStartY = useRef<number | null>(null)

  if (!open) return null

  const handleTouchStart = (e: TouchEvent) => {
    touchStartY.current = e.touches[0].clientY
  }

  const handleTouchMove = (e: TouchEvent) => {
    if (touchStartY.current === null) return
    const deltaY = e.touches[0].clientY - touchStartY.current
    if (deltaY > 0) {
      setDragY(deltaY)
    }
  }

  const handleTouchEnd = () => {
    if (touchStartY.current === null) return
    if (dragY > 100) {
      onClose()
    }
    setDragY(0)
    touchStartY.current = null
  }

  return (
    <div onClick={onClose} className={s.overlay}>
      <div
        onClick={(e: Event) => e.stopPropagation()}
        className={s.sheet}
        style={dragY > 0 ? { transform: `translateY(${dragY}px)`, transition: 'none' } : undefined}
      >
        {/* Drag handle */}
        <div
          className={s.handleBar}
          onTouchStart={handleTouchStart}
          onTouchMove={handleTouchMove}
          onTouchEnd={handleTouchEnd}
        >
          <div className={s.handle} />
        </div>
        {children}
      </div>
    </div>
  )
}
