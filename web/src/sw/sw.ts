/// <reference lib="webworker" />
import { precacheAndRoute } from 'workbox-precaching';
import { registerRoute } from 'workbox-routing';
import { CacheFirst, StaleWhileRevalidate } from 'workbox-strategies';
import { ExpirationPlugin } from 'workbox-expiration';
import type { SWMessage } from './workerTypes';
import { matchesProxy, matchesEntries, matchesExcluded, apiCacheName } from './swPatterns';

declare let self: ServiceWorkerGlobalScope;

// precacheAndRoute handles the app shell AND the navigation fallback to
// index.html for deep-linked routes — do NOT add a separate registerRoute
// for navigate requests, as that would shadow the precache handler.
precacheAndRoute(self.__WB_MANIFEST);

let userId: number | null = null;

// Per-user strategy instances, created lazily and reused across requests.
// Workbox ExpirationPlugin maintains internal IDB state — never instantiate
// per-request or expiration tracking won't accumulate correctly.
const proxyStrategies = new Map<number, CacheFirst>();
const apiStrategies = new Map<number, StaleWhileRevalidate>();

function getProxyStrategy(uid: number): CacheFirst {
  if (!proxyStrategies.has(uid)) {
    proxyStrategies.set(uid, new CacheFirst({
      cacheName: apiCacheName('tap-proxy', uid),
      plugins: [new ExpirationPlugin({ maxEntries: 500, maxAgeSeconds: 30 * 24 * 60 * 60 })],
    }));
  }
  return proxyStrategies.get(uid)!;
}

function getApiStrategy(uid: number): StaleWhileRevalidate {
  if (!apiStrategies.has(uid)) {
    apiStrategies.set(uid, new StaleWhileRevalidate({
      cacheName: apiCacheName('tap-api', uid),
      plugins: [new ExpirationPlugin({ maxEntries: 200, maxAgeSeconds: 7 * 24 * 60 * 60 })],
    }));
  }
  return apiStrategies.get(uid)!;
}

// Receive user context and logout signals from the app.
self.addEventListener('message', (event: MessageEvent<SWMessage | { type: string }>) => {
  const data = event.data;
  if (data.type === 'SKIP_WAITING') {
    self.skipWaiting();
  } else if (data.type === 'set-user') {
    userId = (data as SWMessage & { type: 'set-user' }).userId;
  } else if (data.type === 'logout') {
    const uid = (data as SWMessage & { type: 'logout' }).userId;
    caches.delete(apiCacheName('tap-proxy', uid));
    caches.delete(apiCacheName('tap-api', uid));
    proxyStrategies.delete(uid);
    apiStrategies.delete(uid);
    userId = null;
  }
});

// Proxy URLs — cache-first, immutable (M3 sets Cache-Control: immutable).
registerRoute(
  ({ url }) => matchesProxy(url.pathname),
  ({ request, event }) => {
    if (userId === null) return fetch(request);
    return getProxyStrategy(userId).handle({ request, event: event as FetchEvent });
  },
);

// Entry list and subscription endpoints — stale-while-revalidate.
// Excluded: search, opml, sessions (volatile or auth-sensitive).
registerRoute(
  ({ url, request }) =>
    request.method === 'GET' &&
    !matchesExcluded(url.pathname) &&
    matchesEntries(url.pathname),
  ({ request, event }) => {
    if (userId === null) return fetch(request);
    return getApiStrategy(userId).handle({ request, event: event as FetchEvent });
  },
);
