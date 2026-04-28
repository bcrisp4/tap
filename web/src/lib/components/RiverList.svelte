<script lang="ts">
	// Scrollable list of EntryRow children. The density class on the
	// container drives padding and summary visibility via the
	// :global(.density-*) selectors inside EntryRow.svelte.
	//
	// Plan 15 adds three new wires:
	//   - `onOpen`: row click navigates to the reader. Distinct from
	//     `onSelect` — see rowClick() below for the contract.
	//   - `multiSelect` / `selectedIds`: when true, rows show a
	//     checkbox; clicks toggle selection rather than open the reader.
	//   - `onToggleRead` / `onToggleSelect`: per-row affordances bubble
	//     up to the parent which owns the mutation hook + selection set.
	import EntryRow from './EntryRow.svelte';
	import type { Entry, Feed } from '$api/types';

	let {
		entries,
		feeds = [],
		selectedId = null,
		density = 'default',
		showSummary = true,
		multiSelect = false,
		selectedIds = new Set<number>(),
		dimRead = true,
		onSelect = (_id: number) => {},
		onOpen = (_id: number) => {},
		onToggleRead = (_id: number, _read: boolean) => {},
		onToggleSelect = (_id: number, _ev: MouseEvent) => {}
	}: {
		entries: Entry[];
		feeds?: Feed[];
		selectedId?: number | null;
		density?: 'compact' | 'default' | 'comfortable';
		showSummary?: boolean;
		multiSelect?: boolean;
		selectedIds?: Set<number>;
		// Forwarded to EntryRow — `/history` opts out (all-read by
		// definition); the unread river keeps the default dim.
		dimRead?: boolean;
		onSelect?: (id: number) => void;
		onOpen?: (id: number) => void;
		onToggleRead?: (id: number, read: boolean) => void;
		onToggleSelect?: (id: number, ev: MouseEvent) => void;
	} = $props();

	// Build the feed lookup once per render rather than re-scanning the
	// feeds array for every row.
	const feedMap = $derived(new Map(feeds.map((f) => [f.id, f])));

	function rowClick(id: number, ev: MouseEvent) {
		// Shift-click / meta-click → toggle selection (entering multi-
		// select mode if we're not in it). Otherwise: in multi-select
		// mode toggle selection; in normal mode update selection AND
		// open the reader. The two callbacks have distinct contracts —
		// onSelect is "make this the keyboard-nav selectedId" (no
		// navigation); onOpen is "navigate to the reader". Desktop
		// passes both (it tracks selection separately for keyboard
		// nav); mobile passes only onOpen because there's no keyboard
		// selection to maintain.
		if (ev.shiftKey || ev.metaKey || ev.ctrlKey) {
			onToggleSelect(id, ev);
			return;
		}
		if (multiSelect) {
			onToggleSelect(id, ev);
			return;
		}
		onSelect(id);
		onOpen(id);
	}
</script>

<div class="river density-{density}" data-testid="river">
	{#each entries as e (e.id)}
		<EntryRow
			entry={e}
			feed={feedMap.get(e.feed_id)}
			selected={e.id === selectedId}
			{showSummary}
			{multiSelect}
			multiSelected={selectedIds.has(e.id)}
			{dimRead}
			onclick={(ev) => rowClick(e.id, ev)}
			{onToggleRead}
			{onToggleSelect}
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
