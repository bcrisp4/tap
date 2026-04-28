// Sync TanStack Query's `onlineManager` with the browser's connectivity
// state. The manager auto-listens to `online` / `offline` window events
// once a query subscribes, but it assumes `online = true` at construction
// — so a page that loads while already offline would incorrectly proceed
// as if connected. The initial `setOnline(navigator.onLine)` here closes
// that gap.
//
// When the manager flips back to online, TanStack Query automatically
// resumes any paused mutations from the in-memory + persisted mutation
// cache — see queries.ts (useToggleRead etc.) for the mutation factories
// that benefit. We do not roll our own queue.

import { onlineManager, type QueryClient } from '@tanstack/svelte-query';

export function bindOnlineManager(): () => void {
	if (typeof window === 'undefined') return () => undefined;

	onlineManager.setOnline(navigator.onLine);
	const apply = () => onlineManager.setOnline(navigator.onLine);
	window.addEventListener('online', apply);
	window.addEventListener('offline', apply);
	return () => {
		window.removeEventListener('online', apply);
		window.removeEventListener('offline', apply);
	};
}

// Drain any paused mutations that survived a reload. Paired with the
// `shouldDehydrateMutation` predicate in `query-client.ts`: the
// persister writes paused mutations to IDB on suspend; on next boot
// `persistQueryClient` rehydrates them into the mutation cache, but
// query-core only auto-resumes when `onlineManager` flips offline →
// online. If the user is already online at boot, nothing fires unless
// we kick the cache explicitly.
//
// `resumePausedMutations()` returns a Promise that rejects if any
// resumed mutation throws. We swallow it here because mutation errors
// already trigger the rollback toast inside `onError` — propagating
// the rejection further would just produce console noise without
// adding any user-facing signal.
export function drainPausedMutations(client: QueryClient): void {
	if (typeof window === 'undefined') return;
	client
		.getMutationCache()
		.resumePausedMutations()
		.catch(() => undefined);
}
