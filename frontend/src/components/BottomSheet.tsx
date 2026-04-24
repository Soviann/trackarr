import { useRef, useEffect } from 'preact/hooks'
import type { ComponentChildren } from 'preact'
import s from './BottomSheet.module.css'

interface BottomSheetProps {
  open: boolean
  onClose: () => void
  children: ComponentChildren
}

export function BottomSheet({ open, onClose, children }: BottomSheetProps) {
  const dragYRef = useRef(0)
  const startYRef = useRef<number | null>(null)
  const sheetRef = useRef<HTMLDivElement>(null)
  const prevOverflowRef = useRef<string | null>(null)
  // Stable ref to onClose so effects don't re-run when parent re-renders
  const onCloseRef = useRef(onClose)
  useEffect(() => { onCloseRef.current = onClose })

  // Enhancement 2 — Body scroll lock with unmount failsafe.
  // The per-`open` effect handles the nominal open/close transition. The
  // separate unmount-only effect guarantees the overflow is restored even
  // if the sheet is torn down abruptly (parent error, hot-reload, crash in
  // a child during commit) before the per-`open` cleanup fires.
  useEffect(() => {
    if (!open) return
    prevOverflowRef.current = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    return () => {
      if (prevOverflowRef.current !== null) {
        document.body.style.overflow = prevOverflowRef.current
        prevOverflowRef.current = null
      }
    }
  }, [open])

  useEffect(() => () => {
    if (prevOverflowRef.current !== null) {
      document.body.style.overflow = prevOverflowRef.current
      prevOverflowRef.current = null
    }
  }, [])

  // Enhancement 1 — Android back button closes sheet
  useEffect(() => {
    if (!open) return
    const token = `bottomsheet-${Date.now()}`
    history.pushState({ token }, '')
    const closedViaBackRef = { current: false }
    const onPopState = () => {
      closedViaBackRef.current = true
      onCloseRef.current()
    }
    window.addEventListener('popstate', onPopState)
    return () => {
      window.removeEventListener('popstate', onPopState)
      // If closing normally (tap overlay, drag dismiss) — pop the dummy entry
      if (!closedViaBackRef.current) {
        history.back()
      }
      closedViaBackRef.current = false
    }
  }, [open])

  // Reset drag state when sheet closes
  useEffect(() => {
    if (!open) {
      startYRef.current = null
      dragYRef.current = 0
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
      dragYRef.current = deltaY
      if (sheetRef.current) {
        sheetRef.current.style.transform = `translateY(${deltaY}px)`
        sheetRef.current.style.transition = 'none'
      }
    }
  }

  const handlePointerUp = () => {
    if (startYRef.current === null) return
    if (dragYRef.current > 100) {
      onCloseRef.current()
    }
    if (sheetRef.current) {
      sheetRef.current.style.transform = ''
      sheetRef.current.style.transition = ''
    }
    dragYRef.current = 0
    startYRef.current = null
  }

  const handlePointerCancel = () => {
    startYRef.current = null
    if (sheetRef.current) {
      sheetRef.current.style.transform = ''
      sheetRef.current.style.transition = ''
    }
    dragYRef.current = 0
  }

  return (
    <div onClick={onClose} className={s.overlay}>
      <div
        ref={sheetRef}
        onClick={(e: Event) => e.stopPropagation()}
        className={s.sheet}
        onPointerDown={handlePointerDown}
        onPointerMove={handlePointerMove}
        onPointerUp={handlePointerUp}
        onPointerCancel={handlePointerCancel}
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
