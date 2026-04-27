// TanStack Query factories — one helper per resource. Centralising
// the query keys here keeps cache invalidation predictable across
// pages.

import { createQuery, createMutation, useQueryClient } from '@tanstack/svelte-query';
import { getList, getJSON, putJSON } from './client';
import type { Entry, Feed, Category, SystemStatus } from './types';

// Query keys are namespaced with an explicit 'list' / 'byId' segment so
// prefix-based filters (e.g. `setQueriesData({queryKey: ['entries','list']})`)
// don't accidentally hit single-entry caches whose value shape differs.
export const keys = {
	feeds: () => ['feeds', 'list'] as const,
	feed: (id: number) => ['feeds', 'byId', id] as const,
	categories: () => ['categories', 'list'] as const,
	entriesAll: () => ['entries'] as const,
	entriesList: () => ['entries', 'list'] as const,
	entries: (params: Record<string, string | number | undefined>) =>
		['entries', 'list', params] as const,
	entry: (id: number) => ['entries', 'byId', id] as const,
	status: () => ['system', 'status'] as const
};

export function useFeeds() {
	return createQuery(() => ({
		queryKey: keys.feeds(),
		queryFn: () => getList<Feed>('/feeds')
	}));
}

export function useEntries(
	params: Record<string, string | number | undefined> = { status: 'unread' }
) {
	return createQuery(() => ({
		queryKey: keys.entries(params),
		queryFn: () => {
			const qs = new URLSearchParams(
				Object.entries(params)
					.filter(([, v]) => v !== undefined)
					.map(([k, v]) => [k, String(v)])
			).toString();
			return getList<Entry>('/entries' + (qs ? '?' + qs : ''));
		}
	}));
}

export function useEntry(id: number) {
	return createQuery(() => ({
		queryKey: keys.entry(id),
		queryFn: () => getJSON<Entry>(`/entries/${id}`)
	}));
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
