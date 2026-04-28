<script lang="ts">
	import { onMount } from 'svelte';
	import { QueryClientProvider } from '@tanstack/svelte-query';
	import { makeQueryClient } from '$lib/query-client';
	import { theme, applyThemeClasses } from '$lib/theme.svelte';
	import { bindOnlineManager, drainPausedMutations } from '$lib/offline/online';
	import { prefetchRecent } from '$lib/offline/prefetch';
	import { hotkeysModal } from '$lib/hotkeys-modal.svelte';
	import { bindInputMode } from '$lib/inputmode.svelte';
	import HotkeysModal from '$lib/components/HotkeysModal.svelte';
	import Toast from '$lib/components/Toast.svelte';
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

	// Track which input device the user is driving the UI with so
	// keyboard-only affordances (the row-selection highlight) hide
	// while the user is on mouse / touch. One layout-wide listener
	// keeps this off the per-component hot path.
	onMount(() => bindInputMode());

	// PWA / offline wiring. adapter-static doesn't auto-register the
	// service worker (no SSR hook), so we do it manually. Initial
	// prefetch is delayed so it doesn't compete with first paint;
	// reconnect refreshes a smaller slice.
	onMount(() => {
		if ('serviceWorker' in navigator) {
			navigator.serviceWorker.register('/service-worker.js').catch(() => undefined);
		}
		const unbindOnline = bindOnlineManager();
		// Replay any mutations that paused while offline before the last
		// reload. Safe to call even if the cache is empty — it's a no-op
		// when the mutation cache contains no paused entries.
		drainPausedMutations(client);
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

	// Global `?` toggle. We mirror the suppression rule used by the
	// per-route handlers in $lib/keyboard.svelte.ts (no shortcuts while
	// typing) so this stays consistent with the rest of the app.
	function onGlobalKey(ev: KeyboardEvent) {
		if (ev.key !== '?') return;
		const t = ev.target as HTMLElement | null;
		if (
			t &&
			(t.tagName === 'INPUT' || t.tagName === 'TEXTAREA' || t.isContentEditable)
		) {
			return;
		}
		ev.preventDefault();
		hotkeysModal.toggle();
	}
</script>

<svelte:window onkeydown={onGlobalKey} />

<QueryClientProvider {client}>
	{@render children()}
	<HotkeysModal />
	<Toast />
</QueryClientProvider>
