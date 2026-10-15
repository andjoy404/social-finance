// Social Finance Web Push Service Worker

self.addEventListener('push', (event) => {
  if (!event.data) {
    return
  }

  try {
    const data = event.data.json()
    const title = data.title || 'Social Finance'
    const options = {
      body: data.body || '',
      icon: '/favicon.ico',
      badge: '/favicon-32x32.png',
      data: data.payload || {},
    }

    event.waitUntil(self.registration.showNotification(title, options))
  } catch (err) {
    const text = event.data.text()
    event.waitUntil(
      self.registration.showNotification('Social Finance', {
        body: text,
        icon: '/favicon.ico',
      })
    )
  }
})

self.addEventListener('notificationclick', (event) => {
  event.notification.close()

  const targetUrl = event.notification.data?.url || '/'

  event.waitUntil(
    clients.matchAll({ type: 'window', includeUncontrolled: true }).then((windowClients) => {
      for (const client of windowClients) {
        if (client.url.includes(targetUrl) && 'focus' in client) {
          return client.focus()
        }
      }
      if (clients.openWindow) {
        return clients.openWindow(targetUrl)
      }
    })
  )
})

