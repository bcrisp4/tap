// Bind TanStack Query's `onlineManager` to the browser's online /
// offline events. While `onlineManager` defaults to listening on the
// window itself, calling it explicitly here gives us a single place to
// extend the wiring later (e.g. forcing online=false during e2e tests
// or pausing replay for backoff windows).
//
// When the manager flips back to online, TanStack Query automatically
// resumes any paused mutations from the in-memory + persisted mutation
// cache — see queries.ts (useToggleRead etc.) for the mutation factories
// that benefit. We do not roll our own queue.

import { onlineManager } from '@tanstack/svelte-query';

export function bindOnlineManager(): () => void {
	if (typeof window === 'undefined') return () => undefined;

	const apply = () => onlineManager.setOnline(navigator.onLine);
	apply();
	window.addEventListener('online', apply);
	window.addEventListener('offline', apply);
	return () => {
		window.removeEventListener('online', apply);
		window.removeEventListener('offline', apply);
	};
}
