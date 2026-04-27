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

// $service-worker exposes hashed JS/CSS chunks (`build`) and static
// assets under `static/` (`files`), but adapter-static does NOT include
// the SPA fallback index.html in either array. We add it explicitly so
// the worker can serve it for any navigation while offline — without
// it, the shell handler hits the network-fail path and returns 503.
const FALLBACK_INDEX = '/';
const SHELL = [...build, ...files, FALLBACK_INDEX];

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
	// Hashed asset URLs match an exact cache entry; navigation requests
	// (req.mode === 'navigate') always resolve to index.html so the SPA
	// shell can boot and the client-side router takes over.
	if (req.mode === 'navigate') {
		const fallback = await cache.match(FALLBACK_INDEX);
		if (fallback) {
			// Try the network first so a deployed update lands without a
			// hard reload — fall back to the cached shell on failure.
			try {
				const fresh = await fetch(req);
				if (fresh.ok) {
					cache.put(FALLBACK_INDEX, fresh.clone()).catch(() => undefined);
					return fresh;
				}
			} catch {
				/* fall through to cached fallback */
			}
			return fallback;
		}
	}
	const cached = await cache.match(req);
	if (cached) return cached;
	try {
		return await fetch(req);
	} catch {
		const indexCached = await cache.match(FALLBACK_INDEX);
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

// Concurrency cap when filling the proxy cache. Worst-case the page
// hands us limit (200) × MAX_URLS_PER_ENTRY (32) = 6400 URLs, and
// firing them all at once would saturate the per-origin connection
// pool and pressure the small Go server. A pool of 6 matches Chrome's
// HTTP/1.1 default. Same shape as prefetchRecent's worker pool.
const PREFETCH_CONCURRENCY = 6;

async function prefetchProxy(urls: string[]): Promise<void> {
	if (urls.length === 0) return;
	const cache = await caches.open(PROXY_CACHE);
	let cursor = 0;
	const workers = Array.from(
		{ length: Math.min(PREFETCH_CONCURRENCY, urls.length) },
		async () => {
			while (cursor < urls.length) {
				const u = urls[cursor++];
				try {
					// Skip if already cached — saves bandwidth on reconnect
					// when the same N most-recent entries are re-prefetched.
					if (await cache.match(u)) continue;
					const res = await fetch(u, { cache: 'reload' });
					if (res.ok) await cache.put(u, res.clone());
				} catch {
					/* a single failure shouldn't kill the whole batch */
				}
			}
		}
	);
	await Promise.all(workers);
}

export {};
