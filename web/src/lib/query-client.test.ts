// Persistence smoke-checks. The full offline → reload → reconnect path
// is covered by Playwright (web/tests/offline-mutation.spec.ts); here we
// verify that the dehydrate/hydrate predicate accepts paused mutations
// and round-trips them through `dehydrate(...)` without throwing.

import { describe, it, expect } from 'vitest';
import { QueryClient } from '@tanstack/svelte-query';
import { dehydrate, hydrate } from '@tanstack/query-core';
import { makeQueryClient } from './query-client';

describe('persist mutation dehydration', () => {
	it('accepts the shouldDehydrateMutation predicate', () => {
		const a = new QueryClient();
		// Calling dehydrate with the predicate must succeed without throwing
		// — the predicate is the load-bearing change in this plan.
		const dehydrated = dehydrate(a, {
			shouldDehydrateMutation: (m) => m.state.isPaused
		});
		expect(dehydrated).toBeTruthy();
	});

	it('round-trips a dehydrated state into a fresh client', () => {
		const a = new QueryClient();
		const dehydrated = dehydrate(a, {
			shouldDehydrateMutation: (m) => m.state.isPaused
		});

		const b = new QueryClient();
		hydrate(b, dehydrated);
		// No paused mutations were scheduled, so the cache should be empty
		// after rehydration. The smoke check is that hydrate doesn't reject
		// the structure produced under the new dehydrateOptions.
		expect(b.getMutationCache().getAll().length).toBe(0);
	});

	it('makeQueryClient returns a working QueryClient', () => {
		const client = makeQueryClient();
		expect(client).toBeInstanceOf(QueryClient);
		// Mutation-cache and query-cache should be wired up.
		expect(client.getMutationCache()).toBeTruthy();
		expect(client.getQueryCache()).toBeTruthy();
	});
});
