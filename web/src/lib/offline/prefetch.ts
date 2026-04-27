// Recent-entry prefetcher. Plan 12's offline reading promise leans on
// two layers:
//
//  1. The service worker (`src/service-worker.ts`) holds three caches:
//     `tap-shell-<version>` (cache-first), `tap-api` (SWR), `tap-proxy`
//     (cache-first).
//  2. TanStack Query's persister (`src/lib/query-client.ts`) keeps the
//     query cache in IndexedDB so list / single-entry data hydrates
//     before any network fetch fires.
//
// On first paint and on every `online` event we walk the most recent N
// unread entries, look at the rendered HTML for `/api/v1/proxy/...`
// references, and ask the SW to prefill `tap-proxy` for them. The
// entry list / detail responses are picked up by the SW SWR strategy
// transparently — no extra plumbing needed there. We only need to
// hand-feed the proxy URLs because they live inside HTML strings that
// the SW can't see until something requests them.

import type { Entry } from '$api/types';
import { getJSON, getList } from '$api/client';

// Tap's proxy URL shape: /api/v1/proxy/<base64url-token>. The
// character set is conservative on purpose — we want to skip anything
// the server would reject as malformed without paying for a fetch.
// The lookahead requires an HTML/text boundary after the token so that
// a stray character (e.g. `/api/v1/proxy/should~not`) doesn't get
// truncated into a valid-looking but bogus URL — that would cost a
// pointless fetch on every prefetch run.
const PROXY_RE = /\/api\/v1\/proxy\/[A-Za-z0-9_\-=.]+(?=["'\s>]|$)/g;

// Cap the number of proxy URLs we collect per entry. The reader rarely
// needs more than a handful of images cached for offline use, and an
// adversarial article shouldn't be able to balloon the cache budget.
const MAX_URLS_PER_ENTRY = 32;

// Concurrency cap when fetching `/entries/{id}` details during a
// prefetch. The list endpoint doesn't include `content`, so we have to
// hit every entry — but firing all 200 in parallel saturates the
// browser's connection pool and pressures the small Go server. A pool
// of 6 matches Chrome's per-origin HTTP/1.1 default.
const FETCH_CONCURRENCY = 6;

// Extract proxy URLs from a single entry's rendered HTML. Exported so
// the unit test can exercise it directly without a fetch round-trip.
export function extractProxyURLs(html: string | null | undefined): string[] {
	if (!html) return [];
	const out: string[] = [];
	for (const m of html.matchAll(PROXY_RE)) {
		out.push(m[0]);
		if (out.length >= MAX_URLS_PER_ENTRY) break;
	}
	return out;
}

export async function prefetchRecent(limit = 200): Promise<void> {
	if (typeof window === 'undefined' || !navigator.onLine) return;
	if (!('serviceWorker' in navigator)) return;

	const list = await getList<Entry>(`/entries?status=unread&limit=${limit}`);
	const ids = list.data.map((e) => e.id);

	const proxyURLs = new Set<string>();
	let cursor = 0;
	const workers = Array.from({ length: Math.min(FETCH_CONCURRENCY, ids.length) }, async () => {
		while (cursor < ids.length) {
			const id = ids[cursor++];
			try {
				const e = await getJSON<Entry>(`/entries/${id}`);
				for (const u of extractProxyURLs(e.content)) proxyURLs.add(u);
			} catch {
				/* a single failure shouldn't kill the prefetch budget */
			}
		}
	});
	await Promise.all(workers);

	if (proxyURLs.size === 0) return;
	const reg = await navigator.serviceWorker.ready;
	reg.active?.postMessage({ type: 'prefetch-proxy', urls: Array.from(proxyURLs) });
}
