import { useState, useRef, useEffect, useCallback } from 'preact/hooks'
import type { RefObject, JSX } from 'preact'

export interface UseSwipeDownToCloseOptions {
  open: boolean
  onClose: () => void
  threshold?: number
  shouldIgnore?: (target: EventTarget | null) => boolean
}

export interface UseSwipeDownToCloseResult<T extends HTMLElement = HTMLDivElement> {
  ref: RefObject<T>
  dragY: number
  style: JSX.CSSProperties | undefined
}

export function useSwipeDownToClose<T extends HTMLElement = HTMLDivElement>({
  open,
  onClose,
  threshold = 100,
  shouldIgnore,
}: UseSwipeDownToCloseOptions): UseSwipeDownToCloseResult<T> {
  const containerRef = useRef<T>(null)
  const [dragY, setDragY] = useState(0)
  const dragYRef = useRef(0)
  const touchStartY = useRef<number | null>(null)
  const openRef = useRef(open)
  const onCloseRef = useRef(onClose)
  const shouldIgnoreRef = useRef(shouldIgnore)

  useEffect(() => {
    openRef.current = open
  }, [open])

  useEffect(() => {
    onCloseRef.current = onClose
  }, [onClose])

  useEffect(() => {
    shouldIgnoreRef.current = shouldIgnore
  }, [shouldIgnore])

  useEffect(() => {
    dragYRef.current = dragY
  }, [dragY])

  const handleTouchStart = useCallback((e: TouchEvent) => {
    if (!openRef.current) return
    if (shouldIgnoreRef.current?.(e.target)) return
    touchStartY.current = e.touches[0].clientY
  }, [])

  const handleTouchMove = useCallback((e: TouchEvent) => {
    if (touchStartY.current === null) return
    const deltaY = e.touches[0].clientY - touchStartY.current
    if (deltaY > 0) {
      if (e.cancelable) {
        e.preventDefault()
      }
      setDragY(deltaY)
    }
  }, [])

  const handleTouchEnd = useCallback(() => {
    if (touchStartY.current === null) return
    if (dragYRef.current > threshold) {
      onCloseRef.current()
    }
    setDragY(0)
    touchStartY.current = null
  }, [threshold])

  useEffect(() => {
    const el = containerRef.current
    if (!el) return

    el.addEventListener('touchstart', handleTouchStart, { passive: true })
    el.addEventListener('touchmove', handleTouchMove, { passive: false })
    el.addEventListener('touchend', handleTouchEnd, { passive: true })
    el.addEventListener('touchcancel', handleTouchEnd, { passive: true })

    return () => {
      el.removeEventListener('touchstart', handleTouchStart)
      el.removeEventListener('touchmove', handleTouchMove)
      el.removeEventListener('touchend', handleTouchEnd)
      el.removeEventListener('touchcancel', handleTouchEnd)
    }
  }, [handleTouchStart, handleTouchMove, handleTouchEnd])

  const style = dragY > 0
    ? { transform: `translateY(${dragY}px)`, transition: 'none' }
    : undefined

  return {
    ref: containerRef,
    dragY,
    style,
  }
}
