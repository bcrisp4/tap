<script lang="ts">
	import { onMount } from 'svelte';
	import { QueryClientProvider } from '@tanstack/svelte-query';
	import { makeQueryClient } from '$lib/query-client';
	import { theme, applyThemeClasses } from '$lib/theme.svelte';
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
</script>

<QueryClientProvider {client}>
	{@render children()}
</QueryClientProvider>
