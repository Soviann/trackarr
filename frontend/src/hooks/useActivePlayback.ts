import { useState, useEffect, useCallback, useRef } from 'preact/hooks'
import { apiFetch } from '../api'
import type { ActivePlaybackSession } from '../types'

const POLL_INTERVAL_MS = 15000

export function useActivePlayback() {
  const [session, setSession] = useState<ActivePlaybackSession | null>(null)
  const [dismissedKey, setDismissedKey] = useState<string | null>(null)
  const timerRef = useRef<number | null>(null)

  const fetchSession = useCallback(async () => {
    if (typeof document !== 'undefined' && document.visibilityState !== 'visible') {
      return
    }
    try {
      const data = await apiFetch<ActivePlaybackSession | undefined>('/playback/active')
      if (data && data.title_id) {
        setSession(data)
      } else {
        setSession(null)
      }
    } catch {
      // transient network or server error, keep quiet
    }
  }, [])

  useEffect(() => {
    fetchSession()

    const onVisibilityChange = () => {
      if (document.visibilityState === 'visible') {
        fetchSession()
      }
    }

    document.addEventListener('visibilitychange', onVisibilityChange)
    const interval = window.setInterval(fetchSession, POLL_INTERVAL_MS)

    return () => {
      document.removeEventListener('visibilitychange', onVisibilityChange)
      window.clearInterval(interval)
    }
  }, [fetchSession])

  const sessionKey = session ? `${session.source}:${session.player || ''}:${session.title_id}` : null
  const isDismissed = sessionKey !== null && dismissedKey === sessionKey

  const dismiss = useCallback(() => {
    if (sessionKey) {
      setDismissedKey(sessionKey)
    }
  }, [sessionKey])

  return {
    session: isDismissed ? null : session,
    rawSession: session,
    dismiss,
    refresh: fetchSession,
  }
}
