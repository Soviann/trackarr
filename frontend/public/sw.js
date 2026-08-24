// Empty fetch handler — required by Chrome on Android to consider the app
// installable as a PWA. We stay online-first (no caching strategy) so we let
// the browser handle every request normally by not calling event.respondWith.
self.addEventListener('fetch', () => {})

self.addEventListener('push', (event) => {
  const data = event.data ? event.data.json() : {}
  const title = data.title || 'Trackarr'
  const options = {
    body: data.body || '',
    icon: '/icon-192.png',
    badge: '/favicon-32.png',
    tag: 'trackarr',
    data: { url: data.url || '/' },
  }
  event.waitUntil(
    Promise.all([
      self.registration.showNotification(title, options),
      updateBadge(),
    ])
  )
})

self.addEventListener('notificationclick', (event) => {
  event.notification.close()
  const raw = event.notification.data?.url || '/'
  const target = new URL(raw, self.location.origin)
  const url = target.origin === self.location.origin ? target.pathname : '/'
  event.waitUntil(
    self.clients.matchAll({ type: 'window', includeUncontrolled: true }).then((clients) => {
      for (const client of clients) {
        if (client.url.includes(self.location.origin) && 'focus' in client) {
          client.navigate(url)
          return client.focus()
        }
      }
      return self.clients.openWindow(url)
    })
  )
})

self.addEventListener('activate', (event) => {
  event.waitUntil(updateBadge())
})

async function updateBadge() {
  try {
    const resp = await fetch('/api/titles/review-count')
    if (!resp.ok) return
    const { count } = await resp.json()
    if (count > 0) {
      navigator.setAppBadge(count)
    } else {
      navigator.clearAppBadge()
    }
  } catch {}
}
