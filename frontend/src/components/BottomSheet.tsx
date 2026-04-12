import { useState, useRef, useEffect } from 'preact/hooks'
import type { ComponentChildren } from 'preact'
import s from './BottomSheet.module.css'

interface BottomSheetProps {
  open: boolean
  onClose: () => void
  children: ComponentChildren
}

export function BottomSheet({ open, onClose, children }: BottomSheetProps) {
  const [dragY, setDragY] = useState(0)
  const startYRef = useRef<number | null>(null)
  const sheetRef = useRef<HTMLDivElement>(null)
  // Stable ref to onClose so effects don't re-run when parent re-renders
  const onCloseRef = useRef(onClose)
  useEffect(() => { onCloseRef.current = onClose })

  // Enhancement 2 — Body scroll lock
  useEffect(() => {
    if (!open) return
    const prev = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    return () => { document.body.style.overflow = prev }
  }, [open])

  // Enhancement 1 — Android back button closes sheet
  useEffect(() => {
    if (!open) return
    const token = `bottomsheet-${Date.now()}`
    history.pushState({ token }, '')
    const onPopState = (e: PopStateEvent) => {
      if (e.state?.token === token) {
        onCloseRef.current()
      }
    }
    window.addEventListener('popstate', onPopState)
    return () => {
      window.removeEventListener('popstate', onPopState)
      // If closing normally (not via back), pop the dummy entry
      if (history.state?.token === token) {
        history.back()
      }
    }
  }, [open])

  // Reset drag state when sheet closes
  useEffect(() => {
    if (!open) {
      startYRef.current = null
      setDragY(0)
    }
  }, [open])

  if (!open) return null

  // Enhancement 3 — Drag on entire sheet via pointer events, guarded by scrollTop
  const handlePointerDown = (e: PointerEvent) => {
    const el = sheetRef.current
    if (!el || el.scrollTop > 0) return
    startYRef.current = e.clientY
  }

  const handlePointerMove = (e: PointerEvent) => {
    if (startYRef.current === null) return
    const deltaY = e.clientY - startYRef.current
    if (deltaY > 0) {
      setDragY(deltaY)
    }
  }

  const handlePointerUp = () => {
    if (startYRef.current === null) return
    if (dragY > 100) {
      onCloseRef.current()
    }
    setDragY(0)
    startYRef.current = null
  }

  return (
    <div onClick={onClose} className={s.overlay}>
      <div
        ref={sheetRef}
        onClick={(e: Event) => e.stopPropagation()}
        className={s.sheet}
        style={dragY > 0 ? { transform: `translateY(${dragY}px)`, transition: 'none' } : undefined}
        onPointerDown={handlePointerDown}
        onPointerMove={handlePointerMove}
        onPointerUp={handlePointerUp}
      >
        {/* Drag handle */}
        <div className={s.handleBar}>
          <div className={s.handle} />
        </div>
        {children}
      </div>
    </div>
  )
}
