<script lang="ts">
	// Scrollable list of EntryRow children. The density class on the
	// container drives padding and summary visibility via the
	// :global(.density-*) selectors inside EntryRow.svelte.
	import EntryRow from './EntryRow.svelte';
	import type { Entry, Feed } from '$api/types';

	let {
		entries,
		feeds = [],
		selectedId = null,
		density = 'default',
		showSummary = true,
		onSelect = (_id: number) => {}
	}: {
		entries: Entry[];
		feeds?: Feed[];
		selectedId?: number | null;
		density?: 'compact' | 'default' | 'comfortable';
		showSummary?: boolean;
		onSelect?: (id: number) => void;
	} = $props();

	// Build the feed lookup once per render rather than re-scanning the
	// feeds array for every row.
	const feedMap = $derived(new Map(feeds.map((f) => [f.id, f])));
</script>

<div class="river density-{density}" data-testid="river">
	{#each entries as e (e.id)}
		<EntryRow
			entry={e}
			feed={feedMap.get(e.feed_id)}
			selected={e.id === selectedId}
			{showSummary}
			onclick={() => onSelect(e.id)}
		/>
	{/each}
	{#if entries.length === 0}
		<div class="empty mono">no unread entries</div>
	{/if}
</div>

<style>
	.river {
		flex: 1;
		overflow-y: auto;
		background: var(--bg);
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
