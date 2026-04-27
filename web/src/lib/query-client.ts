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

export function makeQueryClient(): QueryClient {
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

	if (typeof window !== 'undefined') {
		const persister = createAsyncStoragePersister({ storage: idbStorage });
		persistQueryClient({
			queryClient: client,
			persister,
			maxAge: 7 * 24 * 60 * 60 * 1000
		});
	}
	return client;
}
