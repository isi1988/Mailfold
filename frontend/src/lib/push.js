// Browser-side glue for Web Push: registering the service worker, asking
// permission, subscribing/unsubscribing with the browser's PushManager, and
// syncing that subscription with the backend (POST/DELETE
// /api/webmail/push/subscribe). This is the only place that deals with the
// browser APIs directly — everything else just calls isPushSupported() /
// getPushSubscription() / enablePush() / disablePush().
import { wm } from '../api/webmail.js';

// isPushSupported reports whether this browser can do Web Push at all, so
// the UI can hide the whole feature rather than offer a button that would
// just fail.
export function isPushSupported() {
  return typeof window !== 'undefined' && 'serviceWorker' in navigator && 'PushManager' in window;
}

// urlBase64ToUint8Array decodes the VAPID public key (base64url, as the
// backend returns it) into the raw bytes PushManager.subscribe() expects for
// applicationServerKey.
function urlBase64ToUint8Array(base64Url) {
  const padding = '='.repeat((4 - (base64Url.length % 4)) % 4);
  const base64 = (base64Url + padding).replace(/-/g, '+').replace(/_/g, '/');
  const raw = atob(base64);
  const bytes = new Uint8Array(raw.length);
  for (let i = 0; i < raw.length; i++) bytes[i] = raw.charCodeAt(i);
  return bytes;
}

async function registerServiceWorker() {
  return navigator.serviceWorker.register('/sw.js');
}

// getPushSubscription returns the browser's existing PushSubscription for
// this origin, if any, without prompting for anything.
export async function getPushSubscription() {
  if (!isPushSupported()) return null;
  const reg = await navigator.serviceWorker.getRegistration('/sw.js');
  if (!reg) return null;
  return reg.pushManager.getSubscription();
}

// enablePush registers the service worker (if needed), asks for
// notification permission, subscribes with the browser's PushManager using
// the server's VAPID key, and tells the backend about the new subscription.
// Throws if permission is denied or any step fails.
export async function enablePush() {
  if (!isPushSupported()) throw new Error('push not supported');
  const permission = await Notification.requestPermission();
  if (permission !== 'granted') throw new Error('permission denied');

  const { public_key: vapidPublicKey } = await wm.push.vapidPublicKey();
  const reg = await registerServiceWorker();
  await navigator.serviceWorker.ready;
  const sub = await reg.pushManager.subscribe({
    userVisibleOnly: true,
    applicationServerKey: urlBase64ToUint8Array(vapidPublicKey),
  });
  const json = sub.toJSON();
  await wm.push.subscribe({ endpoint: json.endpoint, keys: json.keys });
  return sub;
}

// disablePush unsubscribes the current device's PushSubscription (if any)
// both from the browser and from the backend.
export async function disablePush() {
  const sub = await getPushSubscription();
  if (!sub) return;
  await wm.push.unsubscribe(sub.endpoint);
  await sub.unsubscribe();
}
