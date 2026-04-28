// TanStack Query factories — one helper per resource. Centralising
// the query keys here keeps cache invalidation predictable across
// pages.

import { createQuery, createMutation, useQueryClient } from '@tanstack/svelte-query';
import { deleteResource, getList, getJSON, postJSON, putJSON } from './client';
import type { Entry, Feed, Category, SystemStatus, FeedPatch, DiscoverResult } from './types';

// Query keys are namespaced with an explicit 'list' / 'byId' segment so
// prefix-based filters (e.g. `setQueriesData({queryKey: ['entries','list']})`)
// don't accidentally hit single-entry caches whose value shape differs.
//
// `history` and `search` use their own subtrees rather than re-using
// the generic `entries` list namespace: history is bound to a special
// read_at ordering and search is bound to a query string, so a
// prefix-matching invalidation on entries shouldn't cross-pollinate
// these caches.
export const keys = {
	feeds: () => ['feeds', 'list'] as const,
	feed: (id: number) => ['feeds', 'byId', id] as const,
	categories: () => ['categories', 'list'] as const,
	entriesAll: () => ['entries'] as const,
	entriesList: () => ['entries', 'list'] as const,
	entries: (params: Record<string, string | number | undefined>) =>
		['entries', 'list', params] as const,
	entry: (id: number) => ['entries', 'byId', id] as const,
	history: (limit: number) => ['history', 'list', limit] as const,
	search: (q: string) => ['search', 'list', q] as const,
	status: () => ['system', 'status'] as const
};

export function useFeeds() {
	return createQuery(() => ({
		queryKey: keys.feeds(),
		queryFn: () => getList<Feed>('/feeds')
	}));
}

// `params` may be a static object (most callers) or a getter that the
// query factory re-evaluates on each pass. The getter form lets a
// route component re-key the query when one of its inputs changes
// without remounting — e.g. `/feeds/[id]` navigating to a sibling
// feed_id while the same `+page.svelte` instance stays alive. Without
// it, the closure freezes the initial params and the query keeps
// targeting the stale id.
export function useEntries(
	params:
		| Record<string, string | number | undefined>
		| (() => Record<string, string | number | undefined>) = { status: 'unread' }
) {
	const getParams = typeof params === 'function' ? params : () => params;
	return createQuery(() => {
		const current = getParams();
		return {
			queryKey: keys.entries(current),
			queryFn: () => {
				const qs = new URLSearchParams(
					Object.entries(current)
						.filter(([, v]) => v !== undefined)
						.map(([k, v]) => [k, String(v)])
				).toString();
				return getList<Entry>('/entries' + (qs ? '?' + qs : ''));
			}
		};
	});
}

// `id` may be passed as a plain number (one-shot lookup) or as a getter
// returning a number. The getter form lets a route component re-key the
// query as its `[id]` param changes — without it, the closure freezes
// the initial id and navigating to a sibling entry would keep showing
// the old article.
//
// We capture the id once per factory evaluation so the queryKey and
// queryFn agree even if `getId()` would return a different number when
// it's called again later — TanStack Query may invoke `queryFn` after
// a microtask, and by then a rapid double-navigation could shift the
// underlying rune.
export function useEntry(id: number | (() => number)) {
	const getId = typeof id === 'function' ? id : () => id;
	return createQuery(() => {
		const currentId = getId();
		return {
			queryKey: keys.entry(currentId),
			queryFn: () => getJSON<Entry>(`/entries/${currentId}`)
		};
	});
}

export function useToggleRead() {
	const qc = useQueryClient();
	return createMutation(() => ({
		mutationFn: async ({ id, read }: { id: number; read: boolean }) =>
			await putJSON<Entry>(`/entries/${id}`, { read }),
		// Optimistic update: flip `read` on every cached entries list
		// before the server replies, then revert on error. Scoped to
		// the 'list' namespace so single-entry caches aren't touched.
		onMutate: async ({ id, read }) => {
			await qc.cancelQueries({ queryKey: keys.entriesList() });
			const prev = qc.getQueriesData<{ data: Entry[] }>({ queryKey: keys.entriesList() });
			qc.setQueriesData<{ data: Entry[] }>({ queryKey: keys.entriesList() }, (old) =>
				old ? { ...old, data: old.data.map((e) => (e.id === id ? { ...e, read } : e)) } : old
			);
			// Also reflect the change in the single-entry cache for the
			// reader pane.
			qc.setQueryData<Entry>(keys.entry(id), (old) => (old ? { ...old, read } : old));
			return { prev };
		},
		onError: (_err, _vars, ctx) => {
			ctx?.prev?.forEach(([key, data]) => qc.setQueryData(key, data));
		},
		onSettled: (_data, _err, { id }) => {
			qc.invalidateQueries({ queryKey: keys.entriesAll() });
			qc.invalidateQueries({ queryKey: keys.entry(id) });
		}
	}));
}

export function useToggleSaved() {
	const qc = useQueryClient();
	return createMutation(() => ({
		mutationFn: async ({ id, saved }: { id: number; saved: boolean }) =>
			await putJSON<Entry>(`/entries/${id}`, { saved }),
		onSettled: (_data, _err, { id }) => {
			qc.invalidateQueries({ queryKey: keys.entriesAll() });
			qc.invalidateQueries({ queryKey: keys.entry(id) });
		}
	}));
}

export function useCategories() {
	return createQuery(() => ({
		queryKey: keys.categories(),
		queryFn: () => getList<Category>('/categories')
	}));
}

export function useStatus() {
	return createQuery(() => ({
		queryKey: keys.status(),
		queryFn: () => getJSON<SystemStatus>('/system/status'),
		refetchInterval: 5_000
	}));
}

// `useHistory` reads the dedicated `?order=read_at` shape — entries
// with a `read_at` timestamp surfaced newest-read first. Lives in its
// own query key (NOT keys.entries) so a generic entries-list
// invalidation doesn't trash it; conversely, mutating an entry's read
// state still invalidates `keys.entriesAll()` which prefix-matches
// 'entries' and history is unaffected (history's prefix is 'history').
export function useHistory(opts: { limit?: number } = {}) {
	const limit = opts.limit ?? 50;
	return createQuery(() => ({
		queryKey: keys.history(limit),
		queryFn: () => getList<Entry>(`/entries?status=read&order=read_at&limit=${limit}`)
	}));
}

// `useFeed` accepts a getter so navigating between sibling /feeds/[id]
// routes re-keys the query (same trick `useEntry` uses).
export function useFeed(id: number | (() => number)) {
	const getId = typeof id === 'function' ? id : () => id;
	return createQuery(() => {
		const currentId = getId();
		return {
			queryKey: keys.feed(currentId),
			queryFn: () => getJSON<Feed>(`/feeds/${currentId}`),
			enabled: Number.isFinite(currentId) && currentId > 0
		};
	});
}

// `useSearch` debounces externally — the route owns the rune that
// drives the getter so this hook stays presentation-free. The
// `enabled` gate avoids a pointless network call until there's enough
// text to be a meaningful FTS5 query (the API's MATCH parser also
// rejects single-char tokens, so we save a round-trip).
export function useSearch(query: () => string) {
	return createQuery(() => {
		const q = query().trim();
		return {
			queryKey: keys.search(q),
			queryFn: () => getList<Entry>(`/search?q=${encodeURIComponent(q)}&limit=50`),
			enabled: q.length >= 2
		};
	});
}

// `useUpdateFeed` patches an existing feed. On success we both update
// the byId cache directly (so the route shows the new fields without
// a refetch) and invalidate the feeds list (which carries titles in
// the sidebar).
export function useUpdateFeed() {
	const qc = useQueryClient();
	return createMutation(() => ({
		mutationFn: ({ id, patch }: { id: number; patch: FeedPatch }) =>
			putJSON<Feed>(`/feeds/${id}`, patch),
		onSuccess: (feed) => {
			qc.setQueryData(keys.feed(feed.id), feed);
			void qc.invalidateQueries({ queryKey: keys.feeds() });
		}
	}));
}

// `useDeleteFeed` removes the feed from every cache it appears in —
// the byId one (gone), the feeds list (sidebar/menu), and any
// entries lists (the deleted feed's entries are FK-cascaded).
export function useDeleteFeed() {
	const qc = useQueryClient();
	return createMutation(() => ({
		mutationFn: (id: number) => deleteResource(`/feeds/${id}`),
		onSuccess: (_v, id) => {
			qc.removeQueries({ queryKey: keys.feed(id) });
			void qc.invalidateQueries({ queryKey: keys.feeds() });
			void qc.invalidateQueries({ queryKey: keys.entriesAll() });
		}
	}));
}

// `useDiscoverFeed` POSTs the candidate URL and returns RSS/Atom
// alternates parsed from the page's <link rel="alternate"> tags.
export function useDiscoverFeed() {
	return createMutation(() => ({
		mutationFn: (url: string) => postJSON<DiscoverResult>('/feeds/discover', { url })
	}));
}

// `useRefreshFeed` triggers a manual refresh on the server (sets
// next_poll_at = now). The poller picks it up on its next tick;
// invalidating entriesAll lets the river update once new rows land.
export function useRefreshFeed() {
	const qc = useQueryClient();
	return createMutation(() => ({
		mutationFn: (id: number) => postJSON(`/feeds/${id}/refresh`, {}),
		onSuccess: () => {
			void qc.invalidateQueries({ queryKey: keys.entriesAll() });
		}
	}));
}

// `useSubscribeFeed` POSTs a new feed; the SPA uses the returned id
// to redirect into /feeds/[id].
export function useSubscribeFeed() {
	const qc = useQueryClient();
	return createMutation(() => ({
		mutationFn: (body: { feed_url: string; title?: string; category_id?: number }) =>
			postJSON<{ id: number }>('/feeds', body),
		onSuccess: () => {
			void qc.invalidateQueries({ queryKey: keys.feeds() });
		}
	}));
}
