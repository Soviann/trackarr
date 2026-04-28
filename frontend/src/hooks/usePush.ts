import { useState, useEffect } from 'preact/hooks'
import { apiFetch } from '../api'

function urlBase64ToUint8Array(base64String: string): Uint8Array<ArrayBuffer> {
  const padding = '='.repeat((4 - (base64String.length % 4)) % 4)
  const base64 = (base64String + padding).replace(/-/g, '+').replace(/_/g, '/')
  const raw = atob(base64)
  const buffer = new ArrayBuffer(raw.length)
  const arr = new Uint8Array(buffer)
  for (let i = 0; i < raw.length; i++) arr[i] = raw.charCodeAt(i)
  return arr
}

export function usePush(vapidPublicKey: string | undefined) {
  const [subscribed, setSubscribed] = useState(false)
  const [pushError, setPushError] = useState(false)

  useEffect(() => {
    if (!vapidPublicKey || !('serviceWorker' in navigator) || !('PushManager' in window)) return

    navigator.serviceWorker.ready.then(async (reg) => {
      const existing = await reg.pushManager.getSubscription()
      if (existing) {
        setSubscribed(true)
        return
      }

      if (Notification.permission === 'denied') return

      const permission = await Notification.requestPermission()
      if (permission !== 'granted') return

      const sub = await reg.pushManager.subscribe({
        userVisibleOnly: true,
        applicationServerKey: urlBase64ToUint8Array(vapidPublicKey),
      })

      await apiFetch('/push/subscribe', {
        method: 'POST',
        body: JSON.stringify(sub.toJSON()),
      })
      setSubscribed(true)
    }).catch((err) => {
      console.error('Push subscription failed:', err)
      setPushError(true)
    })
  }, [vapidPublicKey])

  return { subscribed, pushError }
}
