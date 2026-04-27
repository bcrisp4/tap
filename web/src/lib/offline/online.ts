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

import { onlineManager } from '@tanstack/svelte-query';

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
