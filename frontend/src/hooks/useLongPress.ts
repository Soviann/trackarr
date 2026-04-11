import { useRef } from 'preact/hooks'

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

export function useLongPress(options: UseLongPressOptions) {
  const { onLongPress, onClick, threshold = 500, moveTolerance = 10 } = options

  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const startPositionRef = useRef<StartPosition | null>(null)
  const firedRef = useRef(false)

  function clearTimer() {
    if (timerRef.current !== null) {
      clearTimeout(timerRef.current)
      timerRef.current = null
    }
  }

  function onPointerDown(e: PointerEvent) {
    startPositionRef.current = { x: e.clientX, y: e.clientY }
    firedRef.current = false

    timerRef.current = setTimeout(() => {
      firedRef.current = true
      onLongPress(e)
    }, threshold)
  }

  function onPointerMove(e: PointerEvent) {
    if (startPositionRef.current === null) return

    const dx = e.clientX - startPositionRef.current.x
    const dy = e.clientY - startPositionRef.current.y
    const distance = Math.sqrt(dx * dx + dy * dy)

    if (distance > moveTolerance) {
      clearTimer()
    }
  }

  function onPointerUp(e: PointerEvent) {
    if (timerRef.current !== null) {
      clearTimer()
      if (!firedRef.current) {
        onClick?.(e)
      }
    }
    startPositionRef.current = null
  }

  function onPointerCancel() {
    clearTimer()
    startPositionRef.current = null
  }

  function onContextMenu(e: Event) {
    e.preventDefault()
  }

  return { onPointerDown, onPointerUp, onPointerMove, onPointerCancel, onContextMenu }
}
