import { useRef, useState, useCallback, useEffect, useId } from 'preact/hooks'
import type { ComponentChildren } from 'preact'
import { haptic, HAPTIC_SHORT } from '../utils/haptic'
import s from './SwipeActions.module.css'

export interface SwipeAction {
  icon: ComponentChildren
  color: string
  label: string
  onAction: () => void | Promise<void>
  disabled?: boolean
}

export interface SwipeActionsProps {
  actions: SwipeAction[]
  children: ComponentChildren
  disabled?: boolean
  threshold?: number
}

// Custom event name used to coordinate one-at-a-time across instances
const CLOSE_EVENT = 'swipe-actions-close'

type Phase = 'idle' | 'swiping' | 'open' | 'exiting'

export function SwipeActions({ actions, children, disabled = false, threshold }: SwipeActionsProps) {
  const id = useId()
  const containerRef = useRef<HTMLDivElement>(null)
  const contentRef = useRef<HTMLDivElement>(null)
  const actionsRef = useRef<HTMLDivElement>(null)

  // All gesture state in refs to avoid stale closures in window listeners
  const phaseRef = useRef<Phase>('idle')
  const startXRef = useRef(0)
  const startYRef = useRef(0)
  const currentOffsetRef = useRef(0)
  const activePointerRef = useRef<number | null>(null)
  const hapticFiredRef = useRef(false)
  const directionLockedRef = useRef<'horizontal' | 'vertical' | null>(null)
  const disabledRef = useRef(disabled)
  const thresholdRef = useRef(threshold)
  const actionsWidthRef = useRef(0)
  const animatingRef = useRef(false)
  const animationTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const skipNextCloseRef = useRef(false)

  // Re-sync props refs each render
  disabledRef.current = disabled
  thresholdRef.current = threshold

  // React state only for what drives re-renders
  const [offset, setOffset] = useState(0)
  const [phase, setPhase] = useState<Phase>('idle')

  const updatePhase = useCallback((p: Phase) => {
    phaseRef.current = p
    setPhase(p)
  }, [])

  const applyOffset = useCallback((x: number) => {
    currentOffsetRef.current = x
    setOffset(x)
  }, [])

  // Compute the actions panel width (measured once per gesture start or lazily)
  const getActionsWidth = useCallback(() => {
    if (actionsWidthRef.current > 0) return actionsWidthRef.current
    if (actionsRef.current) {
      // Sum of individual button widths (or container scroll width)
      const w = actionsRef.current.scrollWidth
      actionsWidthRef.current = w
      return w
    }
    return actions.length * 80
  }, [actions.length])

  const getThreshold = useCallback(() => {
    const explicit = thresholdRef.current
    if (explicit !== undefined) return explicit
    const containerWidth = containerRef.current?.offsetWidth ?? 0
    return containerWidth * 0.4
  }, [])

  // Close this instance (animate back to idle)
  const close = useCallback((animate = true) => {
    if (phaseRef.current === 'idle') return
    if (animate) animatingRef.current = true
    applyOffset(0)
    updatePhase('idle')
    if (animationTimerRef.current !== null) clearTimeout(animationTimerRef.current)
    animationTimerRef.current = setTimeout(() => { animatingRef.current = false }, 220)
  }, [applyOffset, updatePhase])

  // Open (snap to actions-width reveal)
  const open = useCallback(() => {
    const w = getActionsWidth()
    if (animationTimerRef.current !== null) clearTimeout(animationTimerRef.current)
    animatingRef.current = true
    applyOffset(-w)
    updatePhase('open')
    animationTimerRef.current = setTimeout(() => { animatingRef.current = false }, 220)
    skipNextCloseRef.current = true
    // Notify all other instances to close
    document.dispatchEvent(new CustomEvent(CLOSE_EVENT, { detail: { id } }))
  }, [id, applyOffset, updatePhase, getActionsWidth])

  // Listen for close broadcasts from other instances
  useEffect(() => {
    const handler = (e: Event) => {
      const ce = e as CustomEvent<{ id: string }>
      if (ce.detail.id !== id) {
        close(true)
      }
    }
    document.addEventListener(CLOSE_EVENT, handler)
    return () => document.removeEventListener(CLOSE_EVENT, handler)
  }, [id, close])

  // Close on outside tap
  useEffect(() => {
    if (phase !== 'open') return
    const handler = (e: PointerEvent) => {
      if (skipNextCloseRef.current) {
        skipNextCloseRef.current = false
        return
      }
      if (!containerRef.current?.contains(e.target as Node)) {
        close(true)
      }
    }
    if (typeof document === 'undefined') return
    document.addEventListener('pointerdown', handler)
    return () => {
      if (typeof document !== 'undefined') {
        document.removeEventListener('pointerdown', handler)
      }
    }
  }, [phase, close])

  const handlePointerDown = useCallback((e: PointerEvent) => {
    if (disabledRef.current) return
    if (activePointerRef.current !== null) return
    if (phaseRef.current === 'exiting') return

    // Clear any pending animation timer so animatingRef is accurate immediately
    if (animationTimerRef.current !== null) {
      clearTimeout(animationTimerRef.current)
      animationTimerRef.current = null
      animatingRef.current = false
    }

    activePointerRef.current = e.pointerId
    startXRef.current = e.clientX
    startYRef.current = e.clientY
    directionLockedRef.current = null
    hapticFiredRef.current = false

    // If already open, capture current offset so drag continues from there
    currentOffsetRef.current = phaseRef.current === 'open' ? -getActionsWidth() : 0
    // Re-measure actions width on each new gesture
    actionsWidthRef.current = actionsRef.current?.scrollWidth ?? 0
  }, [getActionsWidth])

  const handlePointerMove = useCallback((e: PointerEvent) => {
    if (disabledRef.current) return
    if (activePointerRef.current !== e.pointerId) return
    if (phaseRef.current === 'exiting') return

    const dx = e.clientX - startXRef.current
    const dy = e.clientY - startYRef.current

    // Dead zone — wait 15px before deciding direction
    if (directionLockedRef.current === null) {
      if (Math.abs(dx) < 15 && Math.abs(dy) < 15) return
      directionLockedRef.current = Math.abs(dx) >= Math.abs(dy) ? 'horizontal' : 'vertical'
    }

    if (directionLockedRef.current === 'vertical') return

    // Prevent browser scroll on horizontal swipe
    e.preventDefault()

    const base = phaseRef.current === 'open' ? -getActionsWidth() : 0
    const raw = base + dx
    // Only allow leftward swipe (negative), clamp at 0 on right side
    // Over-drag limit: 1.25× actions width
    const maxDrag = -getActionsWidth() * 1.25
    const clamped = Math.max(maxDrag, Math.min(0, raw))

    applyOffset(clamped)
    if (phaseRef.current !== 'swiping') updatePhase('swiping')

    // Haptic at threshold crossing
    const threshold = getThreshold()
    if (Math.abs(clamped) >= threshold && !hapticFiredRef.current) {
      haptic(HAPTIC_SHORT)
      hapticFiredRef.current = true
    }
    if (Math.abs(clamped) < threshold) {
      hapticFiredRef.current = false
    }
  }, [applyOffset, updatePhase, getActionsWidth, getThreshold])

  const executeAction = useCallback((action: SwipeAction) => {
    if (action.disabled) return

    // Exit animation: slide out left, then collapse height
    const contentEl = contentRef.current
    const containerEl = containerRef.current
    if (contentEl && containerEl) {
      const height = containerEl.offsetHeight
      updatePhase('exiting')
      applyOffset(-containerEl.offsetWidth)

      // Collapse height after slide
      setTimeout(() => {
        containerEl.style.maxHeight = `${height}px`
        containerEl.style.marginBottom = '0'
        // Force reflow
        containerEl.offsetHeight // eslint-disable-line @typescript-eslint/no-unused-expressions
        containerEl.style.transition = 'max-height 300ms ease, margin-bottom 300ms ease, opacity 200ms ease 150ms'
        containerEl.style.maxHeight = '0'
        containerEl.style.overflow = 'hidden'
        containerEl.style.opacity = '0'
      }, 200)
    }

    // Execute and let parent re-fetch (mutate) remove from DOM
    Promise.resolve(action.onAction()).catch(() => {
      // On error, clear exit animation styles and snap back
      if (containerEl) containerEl.style.cssText = ''
      applyOffset(0)
      updatePhase('idle')
    })
  }, [applyOffset, updatePhase])

  const handlePointerUp = useCallback((e: PointerEvent) => {
    if (activePointerRef.current !== e.pointerId) return
    activePointerRef.current = null

    if (phaseRef.current !== 'swiping') return

    const threshold = getThreshold()
    const currentX = currentOffsetRef.current

    if (Math.abs(currentX) >= threshold) {
      // Past threshold: full reveal or auto-execute primary action
      const actionsWidth = getActionsWidth()
      if (Math.abs(currentX) >= actionsWidth * 1.1) {
        // Far drag — execute primary action automatically
        const primary = actions[0]
        if (primary && !primary.disabled) {
          if (animationTimerRef.current !== null) clearTimeout(animationTimerRef.current)
          animatingRef.current = true
          applyOffset(-actionsWidth)
          updatePhase('open')
          animationTimerRef.current = setTimeout(() => {
            animatingRef.current = false
            executeAction(primary)
          }, 220)
        } else {
          open()
        }
      } else {
        // Snap to reveal
        open()
      }
    } else {
      // Below threshold — snap back
      close(true)
    }
  }, [getThreshold, getActionsWidth, actions, applyOffset, updatePhase, open, close, executeAction])

  const handlePointerCancel = useCallback((e: PointerEvent) => {
    if (activePointerRef.current !== e.pointerId) return
    activePointerRef.current = null
    if (phaseRef.current === 'swiping') {
      close(true)
    }
  }, [close])

  // Attach move/up/cancel on window so drag tracks past container bounds
  useEffect(() => {
    window.addEventListener('pointermove', handlePointerMove, { passive: false })
    window.addEventListener('pointerup', handlePointerUp)
    window.addEventListener('pointercancel', handlePointerCancel)
    return () => {
      window.removeEventListener('pointermove', handlePointerMove)
      window.removeEventListener('pointerup', handlePointerUp)
      window.removeEventListener('pointercancel', handlePointerCancel)
    }
  }, [handlePointerMove, handlePointerUp, handlePointerCancel])

  const isAnimating = animatingRef.current || phase === 'idle' || phase === 'open'

  const contentStyle = {
    transform: `translateX(${offset}px)`,
  }

  return (
    <div
      ref={containerRef}
      class={s.container}
      onPointerDown={handlePointerDown}
    >
      <div
        ref={actionsRef}
        class={s.actions}
        aria-hidden="true"
      >
        {actions.map((action, i) => (
          <button
            key={i}
            class={s.actionBtn}
            style={{ background: action.color }}
            disabled={action.disabled}
            aria-label={action.label}
            onPointerDown={(e: PointerEvent) => e.stopPropagation()}
            onClick={async (e: MouseEvent) => {
              e.stopPropagation()
              close(false)
              executeAction(action)
            }}
          >
            <span class={s.actionIcon}>{action.icon}</span>
            <span class={s.actionLabel}>{action.label}</span>
          </button>
        ))}
      </div>
      <div
        ref={contentRef}
        class={`${s.content} ${isAnimating ? s.animating : ''}`}
        style={contentStyle}
      >
        {children}
      </div>
    </div>
  )
}
