// TanStack Query factories — one helper per resource. Centralising
// the query keys here keeps cache invalidation predictable across
// pages.

import { createQuery, createMutation, useQueryClient } from '@tanstack/svelte-query';
import { getList, getJSON, putJSON, postJSON, deleteResource } from './client';
import type { Entry, Feed, Category, SystemStatus } from './types';

export const keys = {
	feeds: () => ['feeds'] as const,
	feed: (id: number) => ['feeds', id] as const,
	categories: () => ['categories'] as const,
	entries: (params: Record<string, string | number | undefined>) => ['entries', params] as const,
	entry: (id: number) => ['entries', id] as const,
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
			const search = new URLSearchParams();
			for (const [k, v] of Object.entries(params)) {
				if (v !== undefined) search.set(k, String(v));
			}
			return getList<Entry>('/entries' + (search.toString() ? '?' + search.toString() : ''));
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
		// before the server replies, then revert on error.
		onMutate: async ({ id, read }) => {
			await qc.cancelQueries({ queryKey: ['entries'] });
			const prev = qc.getQueriesData<{ data: Entry[] }>({ queryKey: ['entries'] });
			qc.setQueriesData<{ data: Entry[] }>({ queryKey: ['entries'] }, (old) =>
				old ? { ...old, data: old.data.map((e) => (e.id === id ? { ...e, read } : e)) } : old
			);
			return { prev };
		},
		onError: (_err, _vars, ctx) => {
			ctx?.prev?.forEach(([key, data]) => qc.setQueryData(key, data));
		},
		onSettled: () => qc.invalidateQueries({ queryKey: ['entries'] })
	}));
}

export function useToggleSaved() {
	const qc = useQueryClient();
	return createMutation(() => ({
		mutationFn: async ({ id, saved }: { id: number; saved: boolean }) =>
			await putJSON<Entry>(`/entries/${id}`, { saved }),
		onSettled: () => qc.invalidateQueries({ queryKey: ['entries'] })
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

export { postJSON, deleteResource };
