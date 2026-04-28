<script lang="ts">
	// /search — full-text search over entries via the FTS5-backed API.
	//
	// The committed query is mirrored to the URL (?q=…) so the back
	// button restores prior searches and search results are
	// shareable/bookmarkable. SearchBox debounces input by 250 ms;
	// useSearch gates network calls until the trimmed query is at
	// least two characters.
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { useFeeds, useSearch } from '$api/queries';
	import { isMobile } from '$lib/breakpoints.svelte';
	import Sidebar from '$lib/components/Sidebar.svelte';
	import RiverList from '$lib/components/RiverList.svelte';
	import SearchBox from '$lib/components/SearchBox.svelte';
	import MobileTopBar from '$lib/components/MobileTopBar.svelte';
	import MobileTabBar from '$lib/components/MobileTabBar.svelte';
	import Wordmark from '$brand/Wordmark.svelte';

	// `inputText` mirrors keystrokes; `committed` is the trimmed query
	// that drives the actual search.
	//
	// The committed query is mirrored to the URL (?q=…) and — crucially
	// — we drive `committed` *from* `page.url.searchParams` reactively.
	// That way the browser's back/forward buttons (or any external URL
	// change like a deep link) re-sync the input and re-run the query
	// without a remount. SearchBox is debounced (250ms idle) so each
	// `commit()` call corresponds to a real user-completed search, not
	// a keystroke; we therefore push a new history entry per commit so
	// browser back walks through prior searches as advertised.
	let inputText = $state('');
	let committed = $state('');

	// External URL changes (back/forward, deep link) take precedence:
	// resync local state when they don't match what we last committed.
	// This also handles the initial mount — `committed` starts empty
	// and gets seeded from the URL on the first effect pass.
	$effect(() => {
		const urlQ = (page.url.searchParams.get('q') ?? '').trim();
		if (urlQ !== committed) {
			committed = urlQ;
			inputText = urlQ;
		}
	});

	const feeds = useFeeds();
	const results = useSearch(() => committed);

	function commit(q: string) {
		const next = q.trim();
		if (next === committed) return;
		committed = next;
		// Push a new history entry per committed search so the back
		// button walks through prior queries. Debouncing in SearchBox
		// ensures one entry per finished search, not per keystroke.
		const url = new URL(window.location.href);
		if (next) url.searchParams.set('q', next);
		else url.searchParams.delete('q');
		void goto(url.pathname + url.search, { keepFocus: true, noScroll: true });
	}

	const trimmed = $derived(committed.trim());
	const visible = $derived(results.data?.data ?? []);
	const total = $derived(results.data?.pagination.total ?? 0);
	const mobile = $derived(isMobile());
</script>

<svelte:head>
	<title>{trimmed ? `${trimmed} — search — tap` : 'Search — tap'}</title>
</svelte:head>

{#snippet searchPanel()}
	<header class="search-head">
		{#if !mobile}
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

{#if mobile}
	<div class="tap is-mobile">
		<MobileTopBar label="search" count={total} showSearch={false} />
		<div class="m-search-wrap">
			{@render searchPanel()}
		</div>
		<MobileTabBar />
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
</style>
