// Unit tests for query factories. We mostly exercise the query key
// shape and helper invariants here — the network paths are covered by
// the Playwright e2e specs in /web/tests, where a real Tap binary is
// running.

import { describe, it, expect } from 'vitest';
import { keys } from './queries';
import type { FeedPatch } from './types';

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
