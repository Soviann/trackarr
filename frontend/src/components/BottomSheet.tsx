import { useRef, useEffect } from 'preact/hooks'
import type { ComponentChildren } from 'preact'
import s from './BottomSheet.module.css'

interface BottomSheetProps {
  open: boolean
  onClose: () => void
  ariaLabel?: string
  children: ComponentChildren
}

// Standard focusable elements — modals only use button/input/a/textarea/select
const FOCUSABLE_SELECTOR = [
  'a[href]',
  'area[href]',
  'button:not([disabled])',
  'input:not([disabled]):not([type="hidden"])',
  'select:not([disabled])',
  'textarea:not([disabled])',
  '[tabindex]:not([tabindex="-1"])',
].join(',')

export function BottomSheet({ open, onClose, ariaLabel = 'Dialog', children }: BottomSheetProps) {
  const dragYRef = useRef(0)
  const startYRef = useRef<number | null>(null)
  const sheetRef = useRef<HTMLDivElement>(null)
  const prevOverflowRef = useRef<string | null>(null)
  const prevFocusRef = useRef<HTMLElement | null>(null)
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
      // If closing normally (tap overlay, drag dismiss), pop the dummy entry —
      // but only if it's still on top. The Search merge sheet calls route()
      // mid-open, which pushes the destination on top of our dummy ; popping
      // there would yank the user back from the page they just landed on.
      if (!closedViaBackRef.current) {
        const currentState = history.state as { token?: string } | null
        if (currentState && currentState.token === token) {
          history.back()
        }
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

  // a11y — Escape closes, initial focus + restoration, tab focus trap.
  // Ne fight pas le drag-to-dismiss (gesture is pointer-only) ni le popstate
  // (back button → onClose → unmount → ce cleanup restaure le focus).
  useEffect(() => {
    if (!open) return
    const sheet = sheetRef.current
    if (!sheet) return

    prevFocusRef.current = (document.activeElement as HTMLElement | null)

    // Respect autoFocus on a child input — if focus is already inside, leave it.
    if (!sheet.contains(document.activeElement)) {
      const first = sheet.querySelector<HTMLElement>(FOCUSABLE_SELECTOR)
      if (first) {
        first.focus({ preventScroll: true })
      } else {
        sheet.focus({ preventScroll: true })
      }
    }

    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        e.preventDefault()
        onCloseRef.current()
        return
      }
      if (e.key !== 'Tab') return
      const focusables = sheet.querySelectorAll<HTMLElement>(FOCUSABLE_SELECTOR)
      if (focusables.length === 0) {
        e.preventDefault()
        sheet.focus({ preventScroll: true })
        return
      }
      const first = focusables[0]
      const last = focusables[focusables.length - 1]
      const active = document.activeElement
      if (e.shiftKey && (active === first || !sheet.contains(active))) {
        e.preventDefault()
        last.focus({ preventScroll: true })
      } else if (!e.shiftKey && (active === last || !sheet.contains(active))) {
        e.preventDefault()
        first.focus({ preventScroll: true })
      }
    }

    document.addEventListener('keydown', onKeyDown)
    return () => {
      document.removeEventListener('keydown', onKeyDown)
      const prev = prevFocusRef.current
      prevFocusRef.current = null
      if (prev && document.contains(prev)) {
        prev.focus({ preventScroll: true })
      }
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
        role="dialog"
        aria-modal="true"
        aria-label={ariaLabel}
        tabIndex={-1}
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
