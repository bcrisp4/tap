<script lang="ts">
	// /search — full-text search over entries via the FTS5-backed API.
	//
	// The committed query is mirrored to the URL (?q=…) so the back
	// button restores prior searches and search results are
	// shareable/bookmarkable. SearchBox debounces input by 250 ms;
	// useSearch gates network calls until the trimmed query is at
	// least two characters.
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { useFeeds, useSearch } from '$api/queries';
	import Sidebar from '$lib/components/Sidebar.svelte';
	import RiverList from '$lib/components/RiverList.svelte';
	import SearchBox from '$lib/components/SearchBox.svelte';
	import MobileTopBar from '$lib/components/MobileTopBar.svelte';
	import MobileTabBar from '$lib/components/MobileTabBar.svelte';
	import Wordmark from '$brand/Wordmark.svelte';

	// Visible input text (mirrors keystrokes). `committed` is the
	// version that drives the query — debounced by SearchBox.
	const initialQ = page.url.searchParams.get('q') ?? '';
	let inputText = $state(initialQ);
	let committed = $state(initialQ);

	const feeds = useFeeds();
	const results = useSearch(() => committed);

	function commit(q: string) {
		committed = q;
		// Replace the URL silently so back/forward feel natural and
		// every keystroke doesn't push a new history entry.
		const url = new URL(window.location.href);
		if (q) url.searchParams.set('q', q);
		else url.searchParams.delete('q');
		void goto(url.pathname + url.search, { replaceState: true, keepFocus: true, noScroll: true });
	}

	const trimmed = $derived(committed.trim());
	const visible = $derived(results.data?.data ?? []);
	const total = $derived(results.data?.pagination.total ?? 0);

	let isMobile = $state(false);
	onMount(() => {
		const mq = window.matchMedia('(max-width: 720px)');
		const apply = () => {
			isMobile = mq.matches;
		};
		apply();
		mq.addEventListener('change', apply);
		return () => mq.removeEventListener('change', apply);
	});
</script>

<svelte:head>
	<title>{trimmed ? `${trimmed} — search — tap` : 'Search — tap'}</title>
</svelte:head>

{#snippet searchPanel()}
	<header class="search-head">
		{#if !isMobile}
			<Wordmark />
		{/if}
		<SearchBox bind:value={inputText} placeholder="search the river…" onCommit={commit} />
		<p class="meta mono" aria-live="polite">
			{#if trimmed.length < 2}
				type at least two characters
			{:else if results.isFetching}
				searching for "{trimmed}"…
			{:else if results.isError}
				search failed · {results.error?.message ?? 'try again'}
			{:else}
				{total} match{total === 1 ? '' : 'es'} for "{trimmed}"
			{/if}
		</p>
	</header>

	<div class="results-wrap">
		{#if trimmed.length >= 2 && !results.isFetching && visible.length === 0 && !results.isError}
			<p class="empty mono">no entries matched "{trimmed}"</p>
		{:else if visible.length > 0}
			<RiverList
				entries={visible}
				feeds={feeds.data?.data ?? []}
				density="default"
				showSummary={true}
				onSelect={(id) => goto('/entry/' + id)}
			/>
		{/if}
	</div>
{/snippet}

{#if isMobile}
	<div class="tap is-mobile">
		<MobileTopBar unread={total} />
		<div class="m-search-wrap">
			{@render searchPanel()}
		</div>
		<MobileTabBar active="search" />
	</div>
{:else}
	<div class="tap">
		<Sidebar />
		<div class="col">
			{@render searchPanel()}
		</div>
	</div>
{/if}

<style>
	.tap {
		display: flex;
		height: 100vh;
		background: var(--bg);
		color: var(--ink);
		font-family: var(--serif);
	}
	.tap.is-mobile {
		flex-direction: column;
	}
	.col {
		flex: 1;
		display: flex;
		flex-direction: column;
		min-width: 0;
	}
	.m-search-wrap {
		flex: 1;
		display: flex;
		flex-direction: column;
		min-height: 0;
		overflow-y: auto;
	}
	.search-head {
		padding: 32px 32px 18px;
		display: flex;
		flex-direction: column;
		gap: 14px;
		border-bottom: 1px solid var(--rule);
	}
	.search-head :global(.wordmark) {
		font-size: 22px;
	}
	.meta {
		font-size: 11px;
		color: var(--ink-3);
		letter-spacing: 0.04em;
		margin: 0;
	}
	.results-wrap {
		flex: 1;
		overflow-y: auto;
		display: flex;
		flex-direction: column;
		min-height: 0;
	}
	.empty {
		padding: 80px 24px;
		text-align: center;
		color: var(--ink-3);
		font-size: 11px;
		letter-spacing: 0.06em;
		text-transform: uppercase;
	}
	.tap :global(*::-webkit-scrollbar) {
		width: 8px;
		height: 8px;
	}
	.tap :global(*::-webkit-scrollbar-thumb) {
		background: var(--ink-4);
		border-radius: 4px;
	}
	.tap :global(*:focus-visible) {
		outline: 2px solid var(--accent);
		outline-offset: 2px;
	}
</style>
