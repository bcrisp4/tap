// Cross-list cache patch helpers — unit tests against a real
// QueryClient. The patch helpers must traverse every cached
// entries-list query (across the `entries`/`history`/`search` root
// namespaces) and apply the patch in place.

import { describe, it, expect } from 'vitest';
import { QueryClient } from '@tanstack/svelte-query';
import { patchEntryEverywhere, removeEntryEverywhere } from './cache-patch';
import type { Entry } from './types';

function entry(id: number, overrides: Partial<Entry> = {}): Entry {
	return {
		id,
		feed_id: 1,
		user_id: 1,
		hash: `h${id}`,
		title: `e${id}`,
		url: `https://x/${id}`,
		summary: '',
		content: '',
		published_at: null,
		reading_time: 0,
		read: false,
		saved: false,
		extraction_failed: false,
		created_at: 0,
		...overrides
	} as Entry;
}

function seed(qc: QueryClient) {
	qc.setQueryData(['entries', 'list', { status: 'unread' }], {
		data: [entry(1), entry(2), entry(3)],
		pagination: { limit: 50, offset: 0, total: 3 }
	});
	qc.setQueryData(['history', 'list', 100], {
		data: [entry(1, { read: true }), entry(2, { read: true })],
		pagination: { limit: 100, offset: 0, total: 2 }
	});
	qc.setQueryData(['search', 'list', 'foo'], {
		data: [entry(1), entry(3)],
		pagination: { limit: 25, offset: 0, total: 2 }
	});
}

describe('patchEntryEverywhere', () => {
	it('patches the matching entry across entries/history/search', () => {
		const qc = new QueryClient();
		seed(qc);

		patchEntryEverywhere(qc, 1, { saved: true });

		expect(
			(qc.getQueryData(['entries', 'list', { status: 'unread' }]) as { data: Entry[] }).data[0].saved
		).toBe(true);
		expect((qc.getQueryData(['history', 'list', 100]) as { data: Entry[] }).data[0].saved).toBe(
			true
		);
		expect((qc.getQueryData(['search', 'list', 'foo']) as { data: Entry[] }).data[0].saved).toBe(
			true
		);
	});

	it('drops the entry from unread + decrements total when patch sets read:true on an unread list', () => {
		const qc = new QueryClient();
		seed(qc);

		patchEntryEverywhere(qc, 1, { read: true });

		const unread = qc.getQueryData(['entries', 'list', { status: 'unread' }]) as {
			data: Entry[];
			pagination: { total: number };
		};
		expect(unread.data.map((e) => e.id)).toEqual([2, 3]);
		expect(unread.pagination.total).toBe(2);

		// history list keeps the entry, just patched
		const history = qc.getQueryData(['history', 'list', 100]) as { data: Entry[] };
		expect(history.data[0].id).toBe(1);
		expect(history.data[0].read).toBe(true);
	});

	it('drops the entry from a saved=true list when patch sets saved:false', () => {
		const qc = new QueryClient();
		qc.setQueryData(['entries', 'list', { status: 'all', saved: 'true' }], {
			data: [entry(1, { saved: true }), entry(2, { saved: true })],
			pagination: { limit: 50, offset: 0, total: 2 }
		});

		patchEntryEverywhere(qc, 1, { saved: false });

		const list = qc.getQueryData(['entries', 'list', { status: 'all', saved: 'true' }]) as {
			data: Entry[];
			pagination: { total: number };
		};
		expect(list.data.map((e) => e.id)).toEqual([2]);
		expect(list.pagination.total).toBe(1);
	});

	it('is a no-op for ids that do not appear in any list', () => {
		const qc = new QueryClient();
		seed(qc);
		patchEntryEverywhere(qc, 999, { read: true });

		const unread = qc.getQueryData(['entries', 'list', { status: 'unread' }]) as { data: Entry[] };
		expect(unread.data.map((e) => e.id)).toEqual([1, 2, 3]);
	});
});

describe('removeEntryEverywhere', () => {
	it('removes matching entries from every list and decrements totals', () => {
		const qc = new QueryClient();
		seed(qc);

		removeEntryEverywhere(qc, (e) => e.feed_id === 1);

		expect(
			(qc.getQueryData(['entries', 'list', { status: 'unread' }]) as { data: Entry[] }).data
		).toEqual([]);
		expect((qc.getQueryData(['history', 'list', 100]) as { data: Entry[] }).data).toEqual([]);
		expect((qc.getQueryData(['search', 'list', 'foo']) as { data: Entry[] }).data).toEqual([]);
	});
});
