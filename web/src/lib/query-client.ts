// TanStack Query setup with an IndexedDB-backed persister so the
// in-memory cache survives page reloads. Design.md §3 pins this stack
// (TanStack Query + idb persister); see also Plan 12 for the
// service-worker layer that complements it.

import { QueryClient } from '@tanstack/svelte-query';
import { createAsyncStoragePersister } from '@tanstack/query-async-storage-persister';
import {
	persistQueryClient,
	type AsyncStorage
} from '@tanstack/query-persist-client-core';
import { openDB, type IDBPDatabase } from 'idb';
import {
	mutationKeys,
	toggleReadMutationOptions,
	toggleSavedMutationOptions,
	bulkUpdateMutationOptions,
	deleteFeedMutationOptions
} from './api/queries';

const DB_NAME = 'tap';
const STORE_NAME = 'queryCache';

let dbPromise: Promise<IDBPDatabase> | null = null;

function getDB(): Promise<IDBPDatabase> {
	if (!dbPromise) {
		dbPromise = openDB(DB_NAME, 1, {
			upgrade(db) {
				if (!db.objectStoreNames.contains(STORE_NAME)) {
					db.createObjectStore(STORE_NAME);
				}
			}
		});
	}
	return dbPromise;
}

const idbStorage: AsyncStorage<string> = {
	getItem: async (k) => (await getDB()).get(STORE_NAME, k),
	setItem: async (k, v) => {
		await (await getDB()).put(STORE_NAME, v, k);
	},
	removeItem: async (k) => {
		await (await getDB()).delete(STORE_NAME, k);
	}
};

export interface QueryClientBundle {
	client: QueryClient;
	// Resolves once `persistQueryClient` finishes restoring the cache
	// from IDB. Boot wiring should `await` this before resuming paused
	// mutations: otherwise the rehydrated mutation might not exist yet
	// when `resumePausedMutations()` runs.
	restored: Promise<void>;
}

export function makeQueryClient(): QueryClientBundle {
	const client = new QueryClient({
		defaultOptions: {
			queries: {
				staleTime: 30_000,
				gcTime: 24 * 60 * 60 * 1000,
				retry: 1,
				refetchOnWindowFocus: false
			},
			mutations: { retry: 0 }
		}
	});

	// `setMutationDefaults` is what makes paused mutations work across
	// reloads: dehydrated mutations only carry their `mutationKey` +
	// variables (functions don't survive JSON), so when the cache
	// rehydrates, `resumePausedMutations()` looks up the registered
	// defaults to find the actual `mutationFn` / `onMutate` / `onError`.
	// Without these registrations the rehydrated mutation would have no
	// way to fire its PUT or re-apply the optimistic patch.
	client.setMutationDefaults(mutationKeys.toggleRead, toggleReadMutationOptions(client));
	client.setMutationDefaults(mutationKeys.toggleSaved, toggleSavedMutationOptions(client));
	client.setMutationDefaults(mutationKeys.bulkUpdate, bulkUpdateMutationOptions(client));
	client.setMutationDefaults(mutationKeys.deleteFeed, deleteFeedMutationOptions(client));

	let restored: Promise<void> = Promise.resolve();
	if (typeof window !== 'undefined') {
		// Aggressive throttle: the default 1s window can drop the optimistic
		// patch on the floor when a user fires an offline action and reloads
		// the tab almost immediately. 50ms still coalesces bursts during
		// normal reads (every list response triggers a setQueryData) without
		// stranding fast-mutation-then-reload paths.
		const persister = createAsyncStoragePersister({
			storage: idbStorage,
			throttleTime: 50
		});
		// `shouldDehydrateMutation` lets paused mutations (queued while
		// the user was offline) survive a tab reload — without it the
		// cache persister only writes queries, so a mark-read fired
		// offline → tab refreshed offline → reconnect would never replay
		// the PUT. `state.isPaused` is the canonical predicate that
		// query-core sets when `onlineManager` reports offline.
		const [, restorePromise] = persistQueryClient({
			queryClient: client,
			persister,
			maxAge: 7 * 24 * 60 * 60 * 1000,
			dehydrateOptions: {
				shouldDehydrateMutation: (m) => m.state.isPaused
			}
		});
		restored = restorePromise;
	}
	return { client, restored };
}
