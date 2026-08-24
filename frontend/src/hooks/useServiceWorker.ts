import { useEffect } from 'preact/hooks'

export function useServiceWorker(enabled: boolean) {
  useEffect(() => {
    if (!enabled || !('serviceWorker' in navigator)) return
    navigator.serviceWorker.register('/sw.js').catch((err) => {
      console.error('Service worker registration failed:', err)
    })
  }, [enabled])
}
