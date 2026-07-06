// Mailfold's service worker. It exists for two things: (1) satisfying the
// browser's PWA installability criteria (a manifest alone isn't enough —
// Chrome and friends also require a registered service worker), and (2)
// showing a system notification when a Web Push message arrives, which is
// the whole point of subscribing in the first place — this fires even with
// no Mailfold tab open anywhere.

self.addEventListener('install', () => {
  self.skipWaiting();
});

self.addEventListener('activate', event => {
  event.waitUntil(self.clients.claim());
});

// A plain pass-through fetch handler. No offline cache is implemented —
// installability just requires that a fetch handler exists, not that it do
// anything beyond letting the request through to the network as normal.
self.addEventListener('fetch', () => {});

self.addEventListener('push', event => {
  let data = { title: 'New mail', body: 'You have new mail.' };
  if (event.data) {
    try { data = event.data.json(); } catch { /* fall back to the default above */ }
  }
  event.waitUntil(
    self.registration.showNotification(data.title || 'New mail', {
      body: data.body || '',
      icon: '/favicon-512.png',
      badge: '/favicon-32.png',
      tag: 'mailfold-new-mail',
    }),
  );
});

// Clicking the notification focuses an already-open Mailfold tab if there is
// one, or opens a new one at the root otherwise.
self.addEventListener('notificationclick', event => {
  event.notification.close();
  event.waitUntil(
    self.clients.matchAll({ type: 'window', includeUncontrolled: true }).then(clientList => {
      for (const client of clientList) {
        if ('focus' in client) return client.focus();
      }
      if (self.clients.openWindow) return self.clients.openWindow('/');
      return undefined;
    }),
  );
});
