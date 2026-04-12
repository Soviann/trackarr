import { useRef, useState, useCallback, useEffect } from 'preact/hooks'
import type { ComponentChildren } from 'preact'
import { haptic, HAPTIC_SHORT } from '../utils/haptic'
import s from './PullToRefresh.module.css'

const DEFAULT_THRESHOLD = 70
const MAX_PULL = 120

export interface PullToRefreshProps {
  onRefresh: () => Promise<void> | void
  children: ComponentChildren
  disabled?: boolean
  threshold?: number
}

type Phase = 'idle' | 'pulling' | 'ready' | 'refreshing'

function ArrowIcon({ color }: { color: string }) {
  return (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke={color} stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
      <line x1="12" y1="19" x2="12" y2="5" />
      <polyline points="5 12 12 5 19 12" />
    </svg>
  )
}

function SpinnerIcon({ color }: { color: string }) {
  return (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke={color} stroke-width="2.5" stroke-linecap="round">
      <path d="M21 12a9 9 0 1 1-6.219-8.56" />
    </svg>
  )
}

export function PullToRefresh({ onRefresh, children, disabled = false, threshold = DEFAULT_THRESHOLD }: PullToRefreshProps) {
  const [phase, setPhase] = useState<Phase>('idle')
  const [pullY, setPullY] = useState(0)

  // All gesture state lives in refs — avoids stale closure issues in window listeners
  const startYRef = useRef<number | null>(null)
  const refreshingRef = useRef(false)
  const hapticFiredRef = useRef(false)
  const activePointerRef = useRef<number | null>(null)
  const phaseRef = useRef<Phase>('idle')
  const disabledRef = useRef(disabled)
  const thresholdRef = useRef(threshold)
  const onRefreshRef = useRef(onRefresh)

  // Keep refs in sync with props/state
  disabledRef.current = disabled
  thresholdRef.current = threshold
  onRefreshRef.current = onRefresh

  const updatePhase = useCallback((p: Phase) => {
    phaseRef.current = p
    setPhase(p)
  }, [])

  const handlePointerDown = useCallback((e: PointerEvent) => {
    if (refreshingRef.current) return
    if (disabledRef.current) return
    if (window.scrollY > 0) return
    startYRef.current = e.clientY
    hapticFiredRef.current = false
    activePointerRef.current = e.pointerId
  }, [])

  const handlePointerMove = useCallback((e: PointerEvent) => {
    if (disabledRef.current || refreshingRef.current) return
    if (startYRef.current === null) return
    if (activePointerRef.current !== e.pointerId) return
    if (window.scrollY > 0) {
      startYRef.current = null
      setPullY(0)
      updatePhase('idle')
      return
    }

    const delta = e.clientY - startYRef.current
    if (delta <= 0) {
      if (phaseRef.current !== 'idle') {
        setPullY(0)
        updatePhase('idle')
      }
      return
    }

    // Threshold check uses raw delta; visual position uses resistance for rubber-band feel
    const isOverThreshold = delta >= thresholdRef.current
    const resistance = 1 - delta / (MAX_PULL * 2.5)
    const visualY = Math.min(delta * Math.max(resistance, 0.2), MAX_PULL)

    if (isOverThreshold) {
      if (!hapticFiredRef.current) {
        haptic(HAPTIC_SHORT)
        hapticFiredRef.current = true
      }
      updatePhase('ready')
    } else {
      hapticFiredRef.current = false
      updatePhase('pulling')
    }

    setPullY(visualY)
  }, [])

  const handlePointerUp = useCallback(async (e: PointerEvent) => {
    if (disabledRef.current) return
    if (startYRef.current === null) return
    if (activePointerRef.current !== e.pointerId) return

    startYRef.current = null
    activePointerRef.current = null

    if (refreshingRef.current) return

    const wasReady = phaseRef.current === 'ready'
    setPullY(0)
    updatePhase(wasReady ? 'refreshing' : 'idle')

    if (wasReady) {
      refreshingRef.current = true
      try {
        await onRefreshRef.current()
      } finally {
        refreshingRef.current = false
        updatePhase('idle')
      }
    }
  }, [])

  const handlePointerCancel = useCallback(() => {
    startYRef.current = null
    activePointerRef.current = null
    hapticFiredRef.current = false
    if (!refreshingRef.current) {
      setPullY(0)
      updatePhase('idle')
    }
  }, [])

  // Attach move/up/cancel on window so gesture tracks past container bounds
  useEffect(() => {
    window.addEventListener('pointermove', handlePointerMove)
    window.addEventListener('pointerup', handlePointerUp)
    window.addEventListener('pointercancel', handlePointerCancel)
    return () => {
      window.removeEventListener('pointermove', handlePointerMove)
      window.removeEventListener('pointerup', handlePointerUp)
      window.removeEventListener('pointercancel', handlePointerCancel)
    }
  }, [handlePointerMove, handlePointerUp, handlePointerCancel])

  const isVisible = phase !== 'idle'
  const isReady = phase === 'ready'
  const isSpinning = phase === 'refreshing'

  // Indicator top position: follows finger while pulling, locks when refreshing
  const indicatorOffset = isSpinning
    ? 12
    : Math.max(-48, pullY - 48)

  const indicatorStyle = {
    transform: `translateX(-50%) translateY(${indicatorOffset}px)`,
    opacity: isVisible ? 1 : 0,
  }

  return (
    <div
      class={s.container}
      onPointerDown={handlePointerDown}
    >
      {isVisible && (
        <div
          class={`${s.indicator} ${isReady ? s.indicatorReady : ''} ${isSpinning ? s.indicatorSpinning : ''}`}
          style={indicatorStyle}
          aria-hidden="true"
        >
          {isSpinning ? (
            <span class={s.spinner}>
              <SpinnerIcon color="#fff" />
            </span>
          ) : (
            <span class={`${s.arrow} ${isReady ? s.arrowReady : ''}`}>
              <ArrowIcon color={isReady ? '#fff' : '#D0D0D0'} />
            </span>
          )}
        </div>
      )}
      {children}
    </div>
  )
}
