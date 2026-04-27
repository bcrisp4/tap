/// <reference lib="webworker" />
/// <reference types="@sveltejs/kit" />

// SvelteKit picks this file up automatically and emits the compiled
// service worker into the static-adapter output. With adapter-static
// the registered URL is `/service-worker.js` (we register it manually
// from `+layout.svelte` — see Task 3 — because adapter-static cannot
// inject auto-registration via SSR).
//
// Cache strategy:
//   - tap-shell-<version>: app shell (HTML / JS / CSS / fonts) —
//     cache-first, version-bumped per build.
//   - tap-api: /api/v1/entries* — stale-while-revalidate.
//   - tap-proxy: /api/v1/proxy/* — cache-first (the proxy URL is an
//     immutable HMAC token, so caching by URL is safe forever).
//
// Mutation endpoints (PUT/POST/PATCH/DELETE) are passed through
// untouched. The TanStack Query mutation queue (with `onlineManager`
// pause/resume) handles offline writes — the SW must not interfere.
import { build, files, version } from '$service-worker';

declare const self: ServiceWorkerGlobalScope;

const SHELL_CACHE = 'tap-shell-' + version;
const API_CACHE = 'tap-api';
const PROXY_CACHE = 'tap-proxy';

const SHELL = [...build, ...files];

self.addEventListener('install', (event) => {
	event.waitUntil(caches.open(SHELL_CACHE).then((c) => c.addAll(SHELL)));
	self.skipWaiting();
});

self.addEventListener('activate', (event) => {
	event.waitUntil(
		(async () => {
			// Drop stale shell caches from previous builds. tap-api and
			// tap-proxy are not version-suffixed; their entries are
			// transparently refreshed by stale-while-revalidate /
			// cache-first on next access.
			for (const k of await caches.keys()) {
				if (k.startsWith('tap-shell-') && k !== SHELL_CACHE) await caches.delete(k);
			}
			await self.clients.claim();
		})()
	);
});

self.addEventListener('fetch', (event) => {
	const req = event.request;
	if (req.method !== 'GET') return;

	const url = new URL(req.url);
	if (url.origin !== self.location.origin) return;

	// /api/v1/proxy/* — immutable token URL → cache-first.
	if (url.pathname.startsWith('/api/v1/proxy/')) {
		event.respondWith(cacheFirst(req, PROXY_CACHE));
		return;
	}
	// /api/v1/entries* and /api/v1/entries/<id> — SWR.
	if (url.pathname.startsWith('/api/v1/entries')) {
		event.respondWith(staleWhileRevalidate(req, API_CACHE));
		return;
	}
	// Other API → network only (don't cache mutations / status / etc.).
	if (url.pathname.startsWith('/api/v1/')) return;
	// Shell + static assets — cache first, fall back to network, fall
	// back to index.html so the SPA shell loads even offline for an
	// unknown route.
	event.respondWith(shellHandler(req));
});

async function cacheFirst(req: Request, cacheName: string): Promise<Response> {
	const cache = await caches.open(cacheName);
	const cached = await cache.match(req);
	if (cached) return cached;
	const res = await fetch(req);
	if (res.ok) cache.put(req, res.clone()).catch(() => undefined);
	return res;
}

async function staleWhileRevalidate(req: Request, cacheName: string): Promise<Response> {
	const cache = await caches.open(cacheName);
	const cached = await cache.match(req);
	const fetched = fetch(req)
		.then((res) => {
			if (res.ok) cache.put(req, res.clone()).catch(() => undefined);
			return res;
		})
		.catch(() => cached ?? Response.error());
	return cached ?? fetched;
}

async function shellHandler(req: Request): Promise<Response> {
	const cache = await caches.open(SHELL_CACHE);
	const cached = await cache.match(req);
	if (cached) return cached;
	try {
		return await fetch(req);
	} catch {
		// SPA navigation fallback when fully offline — serve index so
		// the client-side router can take over.
		const indexCached = await cache.match('/');
		if (indexCached) return indexCached;
		return new Response('offline', { status: 503 });
	}
}

// Pages can ask the SW to prefill the proxy cache with media URLs they
// just rendered. Keeps the reader fully offline-readable for entries
// the user viewed online.
self.addEventListener('message', (event: ExtendableMessageEvent) => {
	const data = event.data as { type?: string; urls?: string[] } | undefined;
	if (data?.type === 'prefetch-proxy' && Array.isArray(data.urls)) {
		event.waitUntil(prefetchProxy(data.urls));
	}
});

async function prefetchProxy(urls: string[]): Promise<void> {
	const cache = await caches.open(PROXY_CACHE);
	await Promise.all(
		urls.map(async (u) => {
			try {
				// Skip if already cached — saves bandwidth on reconnect
				// when the same N most-recent entries are re-prefetched.
				if (await cache.match(u)) return;
				const res = await fetch(u, { cache: 'reload' });
				if (res.ok) await cache.put(u, res.clone());
			} catch {
				/* ignore */
			}
		})
	);
}

export {};
