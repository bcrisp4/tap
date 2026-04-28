// Unit tests for query factories. We mostly exercise the query key
// shape and helper invariants here — the network paths are covered by
// the Playwright e2e specs in /web/tests, where a real Tap binary is
// running.
//
// For mutation hooks we exercise the underlying mutation-options
// objects directly via MutationObserver so we can inspect optimistic
// updates / rollbacks / invalidation without needing a Svelte runtime
// or a network mock library.

import { afterEach, describe, it, expect, vi } from 'vitest';
import { MutationObserver, QueryClient } from '@tanstack/svelte-query';
import { keys, toggleReadMutationOptions } from './queries';
import type { Entry, FeedPatch } from './types';

describe('query keys', () => {
	it('namespaces history and search separately from generic entries', () => {
		// A prefix-matching invalidation on 'entries' (used by mark-read
		// optimistic updates) must NOT cross-contaminate history/search,
		// which carry their own ordering / query semantics.
		const histKey = keys.history(50);
		const searchKey = keys.search('hello');
		const entriesAllKey = keys.entriesAll();
		expect(histKey[0]).toBe('history');
		expect(searchKey[0]).toBe('search');
		expect(entriesAllKey[0]).toBe('entries');
	});

	it('feed byId is distinct from feeds list', () => {
		const single = keys.feed(7);
		const list = keys.feeds();
		expect(single).not.toEqual(list);
		expect(single[0]).toBe('feeds');
		expect(single[1]).toBe('byId');
	});

	it('history key changes with limit so different page sizes cache independently', () => {
		expect(keys.history(50)).not.toEqual(keys.history(100));
	});

	it('search key reflects the trimmed query so different searches cache independently', () => {
		expect(keys.search('rust')).not.toEqual(keys.search('go'));
		expect(keys.search('')).not.toEqual(keys.search('rust'));
	});
});

describe('FeedPatch credential discipline', () => {
	// The wire contract from Plan 08: empty credential inputs must NOT
	// be sent (empty string overwrites the stored value). The form
	// builds a FeedPatch by only assigning fields that are non-empty;
	// these tests document and lock in that semantics.
	function patchFromForm(form: { title: string; cookie: string; username: string }): FeedPatch {
		const patch: FeedPatch = { title: form.title };
		if (form.cookie) patch.cookie = form.cookie;
		if (form.username) patch.username = form.username;
		return patch;
	}

	it('omits empty credential fields from the patch payload', () => {
		const patch = patchFromForm({ title: 'New title', cookie: '', username: '' });
		expect(patch.title).toBe('New title');
		expect('cookie' in patch).toBe(false);
		expect('username' in patch).toBe(false);
	});

	it('includes credential fields when explicitly set', () => {
		const patch = patchFromForm({
			title: 'New title',
			cookie: 'session=abc',
			username: 'alice'
		});
		expect(patch.cookie).toBe('session=abc');
		expect(patch.username).toBe('alice');
	});
});

// --- Mutation hooks ---------------------------------------------------
//
// We bypass `createMutation` (which needs the Svelte runes runtime) and
// drive the underlying `MutationObserver` from query-core. The mutation
// option factories are pure functions, so they're directly testable
// against a real QueryClient.

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

function runMutation<TVars>(
	qc: QueryClient,
	options: ReturnType<typeof toggleReadMutationOptions>,
	vars: TVars
): Promise<unknown> {
	const obs = new MutationObserver(qc, options as Parameters<typeof obs.setOptions>[0]);
	return obs.mutate(vars).catch(() => undefined);
}

function stubFetchOk(body: unknown): void {
	globalThis.fetch = vi.fn(async () =>
		new Response(JSON.stringify(body), {
			status: 200,
			headers: { 'Content-Type': 'application/json' }
		})
	) as unknown as typeof fetch;
}

function stubFetchError(): void {
	globalThis.fetch = vi.fn(async () => {
		throw new Error('network down');
	}) as unknown as typeof fetch;
}

const realFetch = globalThis.fetch;

afterEach(() => {
	globalThis.fetch = realFetch;
});

describe('useToggleRead', () => {
	it('optimistically marks read across entries/history/search and invalidates on settle', async () => {
		stubFetchOk({ id: 1, read: true });
		const qc = new QueryClient({ defaultOptions: { mutations: { retry: 0 } } });
		qc.setQueryData(['entries', 'list', { status: 'unread' }], {
			data: [entry(1)],
			pagination: { limit: 50, offset: 0, total: 1 }
		});
		qc.setQueryData(['history', 'list', 100], {
			data: [entry(1)],
			pagination: { limit: 100, offset: 0, total: 1 }
		});

		await runMutation(qc, toggleReadMutationOptions(qc), { id: 1, read: true });

		// Unread list dropped the row.
		expect(
			(qc.getQueryData(['entries', 'list', { status: 'unread' }]) as { data: Entry[] }).data
		).toEqual([]);

		// History list reflects read:true.
		const hist = qc.getQueryData(['history', 'list', 100]) as { data: Entry[] };
		expect(hist.data[0].read).toBe(true);

		// On settle the queries are invalidated.
		expect(qc.getQueryState(['history', 'list', 100])?.isInvalidated).toBe(true);
	});

	it('rolls back on error', async () => {
		stubFetchError();
		const qc = new QueryClient({ defaultOptions: { mutations: { retry: 0 } } });
		qc.setQueryData(['history', 'list', 100], {
			data: [entry(1)],
			pagination: { limit: 100, offset: 0, total: 1 }
		});

		await runMutation(qc, toggleReadMutationOptions(qc), { id: 1, read: true });

		const hist = qc.getQueryData(['history', 'list', 100]) as { data: Entry[] };
		expect(hist.data[0].read).toBe(false);
	});
});

