<script lang="ts">
	import { onMount } from 'svelte';
	import { QueryClientProvider } from '@tanstack/svelte-query';
	import { makeQueryClient } from '$lib/query-client';
	import { theme, applyThemeClasses } from '$lib/theme.svelte';
	import { bindOnlineManager } from '$lib/offline/online';
	import { prefetchRecent } from '$lib/offline/prefetch';
	import '../app.css';

	const client = makeQueryClient();

	let { children } = $props();

	// $effect handles initial application + re-runs whenever theme.theme
	// or theme.font changes (resolving 'system' -> light/dark via the
	// prefers-color-scheme media query).
	$effect(() => {
		void theme.theme;
		void theme.font;
		applyThemeClasses();
	});

	// Live OS theme tracking: when the user has theme = 'system', flip
	// classes whenever the OS preference changes.
	onMount(() => {
		const mq = window.matchMedia('(prefers-color-scheme: dark)');
		mq.addEventListener('change', applyThemeClasses);
		return () => mq.removeEventListener('change', applyThemeClasses);
	});

	// Plan 12 — PWA / offline wiring.
	//
	// SvelteKit emits the compiled service worker into the static-adapter
	// output at /service-worker.js. With adapter-static there's no SSR
	// hook to inject auto-registration, so we register manually here.
	// The bind + prefetch run on next paint so they don't block first
	// render; reconnect refreshes the prefetch with a smaller budget.
	onMount(() => {
		if ('serviceWorker' in navigator) {
			navigator.serviceWorker.register('/service-worker.js').catch(() => undefined);
		}
		const unbindOnline = bindOnlineManager();
		const initialPrefetch = window.setTimeout(() => {
			void prefetchRecent(200).catch(() => undefined);
		}, 1_000);
		const onOnline = () => void prefetchRecent(50).catch(() => undefined);
		window.addEventListener('online', onOnline);
		return () => {
			window.removeEventListener('online', onOnline);
			window.clearTimeout(initialPrefetch);
			unbindOnline();
		};
	});
</script>

<QueryClientProvider {client}>
	{@render children()}
</QueryClientProvider>
