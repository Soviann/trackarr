import { createContext } from 'preact'
import { useContext, useState, useEffect, useRef, useCallback } from 'preact/hooks'
import type { ComponentChildren } from 'preact'

export interface UndoAction {
  id?: string
  message: string
  actionLabel?: string
  durationMs?: number
  onUndo: () => void | Promise<void>
  onExpire?: () => void | Promise<void>
}

export interface UndoContextValue {
  activeUndo: UndoAction | null
  progress: number // 1.0 down to 0.0
  remainingSeconds: number
  showUndo: (action: UndoAction) => void
  triggerUndo: () => Promise<void>
  dismiss: () => void
}

const DEFAULT_DURATION_MS = 5000

const UndoContext = createContext<UndoContextValue | null>(null)

export function UndoProvider({ children }: { children: ComponentChildren }) {
  const [activeUndo, setActiveUndo] = useState<UndoAction | null>(null)
  const [progress, setProgress] = useState(1)
  const [remainingSeconds, setRemainingSeconds] = useState(5)

  const activeRef = useRef<UndoAction | null>(null)
  activeRef.current = activeUndo

  const timerRef = useRef<number | null>(null)
  const rafRef = useRef<number | null>(null)
  const startTimeRef = useRef<number>(0)
  const durationRef = useRef<number>(DEFAULT_DURATION_MS)

  const cleanupTimer = useCallback(() => {
    if (timerRef.current !== null) {
      window.clearTimeout(timerRef.current)
      timerRef.current = null
    }
    if (rafRef.current !== null) {
      window.cancelAnimationFrame(rafRef.current)
      rafRef.current = null
    }
  }, [])

  const finalizeCurrent = useCallback(() => {
    const current = activeRef.current
    if (current?.onExpire) {
      try {
        current.onExpire()
      } catch (err) {
        console.error('Error executing undo onExpire:', err)
      }
    }
  }, [])

  const dismiss = useCallback(() => {
    cleanupTimer()
    finalizeCurrent()
    setActiveUndo(null)
    setProgress(1)
  }, [cleanupTimer, finalizeCurrent])

  const triggerUndo = useCallback(async () => {
    cleanupTimer()
    const current = activeRef.current
    setActiveUndo(null)
    setProgress(1)

    if (current?.onUndo) {
      try {
        await current.onUndo()
      } catch (err) {
        console.error('Failed to execute undo action:', err)
      }
    }
  }, [cleanupTimer])

  const showUndo = useCallback(
    (action: UndoAction) => {
      // If an existing undo with an onExpire was active, finalize it first
      if (activeRef.current && activeRef.current !== action) {
        finalizeCurrent()
      }

      cleanupTimer()

      const duration = action.durationMs ?? DEFAULT_DURATION_MS
      durationRef.current = duration
      startTimeRef.current = Date.now()

      setActiveUndo(action)
      setProgress(1)
      setRemainingSeconds(Math.ceil(duration / 1000))

      const tick = () => {
        const elapsed = Date.now() - startTimeRef.current
        const remaining = Math.max(0, duration - elapsed)
        const ratio = Math.max(0, remaining / duration)

        setProgress(ratio)
        setRemainingSeconds(Math.ceil(remaining / 1000))

        if (ratio > 0) {
          rafRef.current = window.requestAnimationFrame(tick)
        }
      }

      rafRef.current = window.requestAnimationFrame(tick)

      timerRef.current = window.setTimeout(() => {
        cleanupTimer()
        finalizeCurrent()
        setActiveUndo(null)
        setProgress(1)
      }, duration)
    },
    [cleanupTimer, finalizeCurrent]
  )

  // Flush pending onExpire on page unload
  useEffect(() => {
    const handleBeforeUnload = () => {
      finalizeCurrent()
    }
    window.addEventListener('beforeunload', handleBeforeUnload)
    return () => {
      window.removeEventListener('beforeunload', handleBeforeUnload)
      cleanupTimer()
    }
  }, [cleanupTimer, finalizeCurrent])

  return (
    <UndoContext.Provider
      value={{
        activeUndo,
        progress,
        remainingSeconds,
        showUndo,
        triggerUndo,
        dismiss,
      }}
    >
      {children}
    </UndoContext.Provider>
  )
}

const defaultUndoContext: UndoContextValue = {
  activeUndo: null,
  progress: 1,
  remainingSeconds: 0,
  showUndo: () => {},
  triggerUndo: async () => {},
  dismiss: () => {},
}

export function useUndo(): UndoContextValue {
  const ctx = useContext(UndoContext)
  return ctx ?? defaultUndoContext
}
