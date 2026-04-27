<script lang="ts">
	import { QueryClientProvider } from '@tanstack/svelte-query';
	import { makeQueryClient } from '$lib/query-client';
	import '../app.css';

	const client = makeQueryClient();

	let { children } = $props();
</script>

<QueryClientProvider {client}>
	{@render children()}
</QueryClientProvider>
