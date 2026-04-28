// TanStack Query factories — one helper per resource. Centralising
// the query keys here keeps cache invalidation predictable across
// pages.

import {
	createQuery,
	createMutation,
	useQueryClient,
	type QueryClient
} from '@tanstack/svelte-query';
import { deleteResource, getList, getJSON, postJSON, putJSON } from './client';
import { patchEntryEverywhere, removeEntryEverywhere } from './cache-patch';
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

// Snapshot of every cached query under the three list-shaped roots that
// `patchEntryEverywhere` walks, plus the per-entry single cache. The
// mutation hooks capture this in `onMutate` so `onError` can restore
// the user's view exactly when the server rejects an optimistic edit.
type ListSnapshot = {
	entries: ReturnType<QueryClient['getQueriesData']>;
	history: ReturnType<QueryClient['getQueriesData']>;
	search: ReturnType<QueryClient['getQueriesData']>;
};

function snapshotLists(qc: QueryClient): ListSnapshot {
	return {
		entries: qc.getQueriesData({ queryKey: ['entries'] }),
		history: qc.getQueriesData({ queryKey: ['history'] }),
		search: qc.getQueriesData({ queryKey: ['search'] })
	};
}

function restoreLists(qc: QueryClient, snap: ListSnapshot): void {
	for (const ns of ['entries', 'history', 'search'] as const) {
		for (const [k, v] of snap[ns]) qc.setQueryData(k, v);
	}
}

async function cancelLists(qc: QueryClient): Promise<void> {
	await qc.cancelQueries({ queryKey: ['entries'] });
	await qc.cancelQueries({ queryKey: ['history'] });
	await qc.cancelQueries({ queryKey: ['search'] });
}

function invalidateLists(qc: QueryClient): void {
	qc.invalidateQueries({ queryKey: ['entries'] });
	qc.invalidateQueries({ queryKey: ['history'] });
	qc.invalidateQueries({ queryKey: ['search'] });
}

// Mutation options for `useToggleRead`, exported as a pure function so
// unit tests can drive the same options through MutationObserver
// without a Svelte runtime. The hook is a thin wrapper that re-resolves
// the client per render via `useQueryClient`.
export function toggleReadMutationOptions(qc: QueryClient) {
	return {
		mutationFn: async ({ id, read }: { id: number; read: boolean }) =>
			await putJSON<Entry>(`/entries/${id}`, { read }),
		onMutate: async ({ id, read }: { id: number; read: boolean }) => {
			await cancelLists(qc);
			const previous = snapshotLists(qc);
			patchEntryEverywhere(qc, id, { read });
			// Reflect the change in the single-entry cache for the reader pane.
			qc.setQueryData<Entry>(keys.entry(id), (old) => (old ? { ...old, read } : old));
			return { previous };
		},
		onError: (_err: unknown, _vars: unknown, ctx: { previous: ListSnapshot } | undefined) => {
			if (!ctx?.previous) return;
			restoreLists(qc, ctx.previous);
		},
		onSettled: (
			_data: unknown,
			_err: unknown,
			vars: { id: number; read: boolean }
		) => {
			invalidateLists(qc);
			qc.invalidateQueries({ queryKey: keys.entry(vars.id) });
		}
	};
}

export function useToggleRead() {
	const qc = useQueryClient();
	return createMutation(() => toggleReadMutationOptions(qc));
}

// `useBulkUpdate` flips `read` on a list of entry ids. (No `saved`
// support today — Plan 15's bulk UI only exposes mark-read / mark-
// unread; extend the vars + mutationFn if a future plan adds bulk
// save/unsave.) The existing PUT /entries/read endpoint is scope-
// based (feed_id / category_id) and doesn't accept a free-form id
// list, so we fan out individual PUT /entries/{id} calls instead.
// Plan 15's multi-select UX never selects more than a screen's worth
// of rows in practice (~50), so the fan-out cost is acceptable.
//
// Optimistic update mirrors useToggleRead: we patch each id across
// every cached list (entries / history / search), snapshot the prior
// state for rollback, and invalidate after settle.
export function bulkUpdateMutationOptions(qc: QueryClient) {
	return {
		mutationFn: async ({ ids, read }: { ids: number[]; read: boolean }) => {
			await Promise.all(ids.map((id) => putJSON<Entry>(`/entries/${id}`, { read })));
		},
		onMutate: async ({ ids, read }: { ids: number[]; read: boolean }) => {
			await cancelLists(qc);
			const previous = snapshotLists(qc);
			for (const id of ids) {
				patchEntryEverywhere(qc, id, { read });
				qc.setQueryData<Entry>(keys.entry(id), (old) => (old ? { ...old, read } : old));
			}
			return { previous };
		},
		onError: (_err: unknown, _vars: unknown, ctx: { previous: ListSnapshot } | undefined) => {
			if (!ctx?.previous) return;
			restoreLists(qc, ctx.previous);
		},
		onSettled: () => {
			invalidateLists(qc);
		}
	};
}

export function useBulkUpdate() {
	const qc = useQueryClient();
	return createMutation(() => bulkUpdateMutationOptions(qc));
}

export function toggleSavedMutationOptions(qc: QueryClient) {
	return {
		mutationFn: async ({ id, saved }: { id: number; saved: boolean }) =>
			await putJSON<Entry>(`/entries/${id}`, { saved }),
		onMutate: async ({ id, saved }: { id: number; saved: boolean }) => {
			await cancelLists(qc);
			const previous = snapshotLists(qc);
			patchEntryEverywhere(qc, id, { saved });
			qc.setQueryData<Entry>(keys.entry(id), (old) => (old ? { ...old, saved } : old));
			return { previous };
		},
		onError: (_err: unknown, _vars: unknown, ctx: { previous: ListSnapshot } | undefined) => {
			if (!ctx?.previous) return;
			restoreLists(qc, ctx.previous);
		},
		onSettled: (
			_data: unknown,
			_err: unknown,
			vars: { id: number; saved: boolean }
		) => {
			invalidateLists(qc);
			qc.invalidateQueries({ queryKey: keys.entry(vars.id) });
		}
	};
}

export function useToggleSaved() {
	const qc = useQueryClient();
	return createMutation(() => toggleSavedMutationOptions(qc));
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
// entries lists (the deleted feed's entries are FK-cascaded). The
// optimistic `removeEntryEverywhere` walk drops the entries from
// every cached list namespace synchronously so the UI doesn't show
// stale rows during the server roundtrip; `onSettled` invalidates
// as a backstop in case the cache held entries we hadn't paged.
export function deleteFeedMutationOptions(qc: QueryClient) {
	return {
		mutationFn: (id: number) => deleteResource(`/feeds/${id}`),
		onMutate: async (id: number) => {
			await cancelLists(qc);
			await qc.cancelQueries({ queryKey: ['feeds'] });

			const previous = {
				...snapshotLists(qc),
				feeds: qc.getQueriesData({ queryKey: ['feeds'] })
			};

			removeEntryEverywhere(qc, (e) => e.feed_id === id);
			qc.removeQueries({ queryKey: keys.feed(id) });
			return { previous };
		},
		onError: (
			_err: unknown,
			_vars: unknown,
			ctx:
				| {
						previous: ListSnapshot & {
							feeds: ReturnType<QueryClient['getQueriesData']>;
						};
				  }
				| undefined
		) => {
			if (!ctx?.previous) return;
			restoreLists(qc, ctx.previous);
			for (const [k, v] of ctx.previous.feeds) qc.setQueryData(k, v);
		},
		onSettled: () => {
			invalidateLists(qc);
			qc.invalidateQueries({ queryKey: keys.feeds() });
		}
	};
}

export function useDeleteFeed() {
	const qc = useQueryClient();
	return createMutation(() => deleteFeedMutationOptions(qc));
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
