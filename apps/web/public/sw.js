// Bal Suraksha — Web Push Service Worker
// Handles push events from the Web Push API (RFC 8030).
//
// Rules:
//   - TEST payloads MUST display "TEST — FICTIONAL" prefix in title and body.
//   - No private case data, return secrets, or session tokens are cached here.
//   - Cache-Control: no-store applies — this SW never caches private content.
//   - The status link opens the authorized public alert status page only.
'use strict';

self.addEventListener('install', (event) => {
  // Skip waiting — activate immediately on install
  self.skipWaiting();
});

self.addEventListener('activate', (event) => {
  // Claim all clients so the SW controls existing tabs immediately
  event.waitUntil(clients.claim());
});

/**
 * Push event: fired when the push service delivers a message.
 * payload.data must be a JSON string matching the server NotificationPayload struct.
 */
self.addEventListener('push', (event) => {
  if (!event.data) {
    console.warn('[sw] Push event received with no data — ignoring.');
    return;
  }

  let payload;
  try {
    payload = event.data.json();
  } catch (e) {
    console.error('[sw] Failed to parse push payload:', e);
    return;
  }

  const isTest = payload.is_test === true;

  // TEST notifications MUST be clearly labeled — delivery plan §2 rule 4
  const title = isTest
    ? `TEST — FICTIONAL: ${payload.title || 'Alert'}`
    : (payload.title || 'Bal Suraksha Alert');

  const body = isTest
    ? `[TEST — FICTIONAL] ${payload.body || 'A verified alert has been issued for your area.'}`
    : (payload.body || 'A verified alert has been issued for your area. Tap to view current status.');

  const options = {
    body,
    icon: '/favicon.svg',
    badge: '/favicon.svg',
    tag: `alert-${payload.alert_id || 'unknown'}`,   // de-dupes notifications for same alert
    renotify: false,                                   // don't re-vibrate on tag update
    requireInteraction: false,
    silent: false,
    data: {
      alertId: payload.alert_id,
      revisionId: payload.revision_id,
      statusUrl: payload.status_url || `/alerts/${payload.alert_id}/status`,
      isTest,
    },
  };

  event.waitUntil(
    self.registration.showNotification(title, options)
  );
});

/**
 * notificationclick: fired when the user taps a push notification.
 * Opens the public alert status page — never a private case URL.
 */
self.addEventListener('notificationclick', (event) => {
  event.notification.close();

  const data = event.notification.data || {};
  // statusUrl is the public current-revision status page — no private data
  const targetUrl = data.statusUrl
    ? new URL(data.statusUrl, self.location.origin).href
    : self.location.origin;

  event.waitUntil(
    clients.matchAll({ type: 'window', includeUncontrolled: true }).then((clientList) => {
      // Focus existing tab if already open
      for (const client of clientList) {
        if (client.url === targetUrl && 'focus' in client) {
          return client.focus();
        }
      }
      // Otherwise open a new tab
      if (clients.openWindow) {
        return clients.openWindow(targetUrl);
      }
    })
  );
});

/**
 * pushsubscriptionchange: fired when the push service rotates the endpoint.
 * The client must re-subscribe and send the new endpoint to the API.
 * We post a message to the controlling page to trigger re-registration.
 */
self.addEventListener('pushsubscriptionchange', (event) => {
  console.warn('[sw] Push subscription changed — client must re-subscribe and update endpoint.');
  event.waitUntil(
    clients.matchAll({ type: 'window' }).then((clientList) => {
      for (const client of clientList) {
        client.postMessage({ type: 'PUSH_SUBSCRIPTION_CHANGED' });
      }
    })
  );
});
