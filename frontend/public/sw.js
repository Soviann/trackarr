self.addEventListener('push', (event) => {
  const data = event.data ? event.data.json() : {}
  const title = data.title || 'PlexTracker'
  const options = {
    body: data.body || '',
    icon: '/icon.png',
    badge: '/favicon-32.png',
    tag: 'plextracker',
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
