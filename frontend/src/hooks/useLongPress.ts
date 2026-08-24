import { useRef, useEffect, useCallback, useMemo } from 'preact/hooks'

export interface UseLongPressOptions {
  onLongPress: (e: PointerEvent) => void
  onClick?: (e: PointerEvent) => void
  threshold?: number
  moveTolerance?: number
}

interface StartPosition {
  x: number
  y: number
}

const DEFAULT_THRESHOLD = 500
const DEFAULT_MOVE_TOLERANCE = 10

export function useLongPress(options: UseLongPressOptions) {
  const optionsRef = useRef(options)
  optionsRef.current = options

  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const startPositionRef = useRef<StartPosition | null>(null)
  const firedRef = useRef(false)

  const clearTimer = useCallback(() => {
    if (timerRef.current !== null) {
      clearTimeout(timerRef.current)
      timerRef.current = null
    }
  }, [])

  // Cancel any pending timer on unmount to prevent stale callback invocations
  useEffect(() => {
    return () => {
      if (timerRef.current !== null) {
        clearTimeout(timerRef.current)
        timerRef.current = null
      }
    }
  }, [])

  const onPointerDown = useCallback((e: PointerEvent) => {
    const { onLongPress, threshold = DEFAULT_THRESHOLD } = optionsRef.current
    startPositionRef.current = { x: e.clientX, y: e.clientY }
    firedRef.current = false

    timerRef.current = setTimeout(() => {
      firedRef.current = true
      timerRef.current = null
      onLongPress(e)
    }, threshold)
  }, [])

  const onPointerMove = useCallback((e: PointerEvent) => {
    if (startPositionRef.current === null) return
    const { moveTolerance = DEFAULT_MOVE_TOLERANCE } = optionsRef.current

    const dx = e.clientX - startPositionRef.current.x
    const dy = e.clientY - startPositionRef.current.y
    const distance = Math.sqrt(dx * dx + dy * dy)

    if (distance > moveTolerance) {
      clearTimer()
    }
  }, [clearTimer])

  const onPointerUp = useCallback((e: PointerEvent) => {
    const { onClick } = optionsRef.current
    if (startPositionRef.current !== null) {
      clearTimer()
      if (!firedRef.current) {
        onClick?.(e)
      }
    }
    startPositionRef.current = null
  }, [clearTimer])

  const onPointerCancel = useCallback(() => {
    clearTimer()
    startPositionRef.current = null
  }, [clearTimer])

  const onContextMenu = useCallback((e: MouseEvent) => {
    e.preventDefault()
  }, [])

  return useMemo(
    () => ({ onPointerDown, onPointerUp, onPointerMove, onPointerCancel, onContextMenu }),
    [onPointerDown, onPointerUp, onPointerMove, onPointerCancel, onContextMenu],
  )
}
