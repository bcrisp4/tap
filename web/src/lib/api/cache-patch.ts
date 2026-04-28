// Cross-list cache mutators. Mutation hooks call these in `onMutate`
// to update every cached entries-list query (across the
// `entries` / `history` / `search` root namespaces) before the
// network roundtrip resolves, so the UI feels instant.
//
// Pagination caveat: dropping a row from the unread list when an
// entry is marked read decrements `total` but does not synthesize
// a new tail — the next refetch repaginates honestly.

import type { QueryClient } from '@tanstack/svelte-query';
import type { Entry } from './types';

type EntryList = {
	data: Entry[];
	pagination: { limit: number; offset: number; total: number };
};

const ROOTS = ['entries', 'history', 'search'] as const;

function isEntryList(value: unknown): value is EntryList {
	if (!value || typeof value !== 'object') return false;
	const v = value as { data?: unknown; pagination?: unknown };
	return Array.isArray(v.data) && !!v.pagination;
}

function shouldDropOnPatch(key: readonly unknown[], patch: Partial<Entry>): boolean {
	// Marking an entry read removes it from any list filtered by status='unread'.
	if (patch.read === true) {
		const params = key[2] as { status?: string } | undefined;
		if (params?.status === 'unread') return true;
	}
	// Unsaving removes the entry from any list filtered by saved='true'.
	if (patch.saved === false) {
		const params = key[2] as { saved?: string } | undefined;
		if (params?.saved === 'true') return true;
	}
	return false;
}

export function patchEntryEverywhere(
	qc: QueryClient,
	id: number,
	patch: Partial<Entry>
): void {
	for (const root of ROOTS) {
		const matches = qc.getQueriesData({ queryKey: [root] });
		for (const [key, value] of matches) {
			if (!isEntryList(value)) continue;

			const drop = shouldDropOnPatch(key, patch);
			let mutated = false;
			const nextData: Entry[] = [];
			for (const e of value.data) {
				if (e.id === id) {
					if (drop) {
						mutated = true;
						continue;
					}
					nextData.push({ ...e, ...patch });
					mutated = true;
				} else {
					nextData.push(e);
				}
			}
			if (!mutated) continue;

			qc.setQueryData(key, {
				data: nextData,
				pagination: {
					...value.pagination,
					total: drop ? Math.max(0, value.pagination.total - 1) : value.pagination.total
				}
			} satisfies EntryList);
		}
	}
}

export function removeEntryEverywhere(
	qc: QueryClient,
	predicate: (e: Entry) => boolean
): void {
	for (const root of ROOTS) {
		const matches = qc.getQueriesData({ queryKey: [root] });
		for (const [key, value] of matches) {
			if (!isEntryList(value)) continue;
			const nextData = value.data.filter((e) => !predicate(e));
			if (nextData.length === value.data.length) continue;
			const removed = value.data.length - nextData.length;
			qc.setQueryData(key, {
				data: nextData,
				pagination: {
					...value.pagination,
					total: Math.max(0, value.pagination.total - removed)
				}
			} satisfies EntryList);
		}
	}
}
