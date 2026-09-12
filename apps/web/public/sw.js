'use strict';
// No fetch cache: private reports and tokens never enter a service-worker cache.
self.addEventListener('install', () => self.skipWaiting());
self.addEventListener('activate', event => event.waitUntil(clients.claim()));
self.addEventListener('push', event => {
  if (!event.data) return;
  let payload;
  try { payload = event.data.json(); } catch { return; }
  if (!/^[a-f0-9-]{36}$/i.test(payload.alert_id || '') || typeof payload.is_test !== 'boolean') return;
  const expiry = Date.parse(payload.expires_at);
  if (!Number.isFinite(expiry) || expiry <= Date.now()) return;
  const title = payload.is_test ? 'TEST — FICTIONAL · Savera Alert' : 'Savera Alert';
  const body = payload.is_test ? 'Fictional test alert for your chosen area. Open to see current status.' : 'An approved missing-child alert is active in your chosen area. Open for current status.';
  event.waitUntil(self.registration.showNotification(title, {
    body, icon:'/savera.svg', badge:'/savera.svg', tag:'savera-'+payload.alert_id,
    renotify:false, data:{alertId:payload.alert_id}
  }));
});
self.addEventListener('notificationclick', event => {
  event.notification.close();
  const id=event.notification.data?.alertId;
  if (!/^[a-f0-9-]{36}$/i.test(id || '')) return;
  // Never open a URL supplied by a notification payload.
  const target=new URL('/alerts/'+id,self.location.origin).href;
  event.waitUntil(clients.matchAll({type:'window',includeUncontrolled:true}).then(windows=>{
    for (const client of windows) if(client.url===target && 'focus' in client)return client.focus();
    return clients.openWindow(target);
  }));
});
self.addEventListener('pushsubscriptionchange', event => {
  event.waitUntil(clients.matchAll({type:'window'}).then(windows=>{
    windows.forEach(client=>client.postMessage({type:'PUSH_SUBSCRIPTION_CHANGED'}));
  }));
});
