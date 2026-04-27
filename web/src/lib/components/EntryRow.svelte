<script lang="ts" module>
	// Module-level helpers: shared between the EntryRow component and
	// any callers (e.g. Sidebar) that need a stable feed-color swatch.
	export function formatAgo(unix?: number | null): string {
		if (!unix) return 'recent';
		const s = Math.max(0, Math.floor(Date.now() / 1000 - unix));
		if (s < 60) return s + 's';
		if (s < 3600) return Math.floor(s / 60) + 'm';
		if (s < 86400) return Math.floor(s / 3600) + 'h';
		return Math.floor(s / 86400) + 'd';
	}

	export function swatchFor(seed: string): string {
		// Deterministic 8-bit hash → HSL hue. Fixed lightness/saturation
		// keeps the swatch palette muted and theme-agnostic.
		let h = 0;
		for (let i = 0; i < seed.length; i++) h = (h * 31 + seed.charCodeAt(i)) & 0xff;
		return `hsl(${(h * 360) / 256} 50% 45%)`;
	}
</script>

<script lang="ts">
	import type { Entry, Feed } from '$api/types';

	let {
		entry,
		feed,
		selected = false,
		showSummary = true,
		onclick = () => {}
	}: {
		entry: Entry;
		feed?: Feed;
		selected?: boolean;
		showSummary?: boolean;
		onclick?: (e: MouseEvent) => void;
	} = $props();

	const ago = $derived(formatAgo(entry.published_at ?? entry.created_at));
	const swatchColor = $derived(swatchFor(feed?.title ?? feed?.feed_url ?? 'tap'));
</script>

<button
	type="button"
	class="entry"
	class:is-read={entry.read}
	class:is-saved={entry.saved}
	class:is-selected={selected}
	{onclick}
>
	<span class="junction" aria-hidden="true"></span>
	{#if entry.saved}
		<span class="saved-mark mono">SAVED</span>
	{/if}
	<h3 class="title">{entry.title}</h3>
	<div class="meta">
		<span class="ico" style="background: {swatchColor}" aria-hidden="true"></span>
		<span class="source">{feed?.title ?? '—'}</span>
		<span class="sep" aria-hidden="true"></span>
		<span class="ago">{ago} ago</span>
		<span class="sep" aria-hidden="true"></span>
		<span class="rt mono">{entry.reading_time} min read</span>
	</div>
	{#if showSummary && entry.summary}
		<p class="summary">{entry.summary}</p>
	{/if}
</button>

<style>
	.entry {
		position: relative;
		display: block;
		text-align: left;
		width: 100%;
		padding: 14px 24px 14px 40px;
		border-bottom: 1px solid var(--rule);
		cursor: pointer;
		background: transparent;
		transition: background 120ms ease;
		font-family: inherit;
		color: inherit;
	}
	.entry:hover {
		background: var(--bg-soft);
	}
	.entry.is-selected {
		background: var(--accent-soft);
	}
	.entry.is-read .title {
		color: var(--ink-3);
		font-weight: 400;
	}
	.entry.is-read .meta {
		color: var(--ink-3);
	}

	.junction {
		position: absolute;
		left: 22px;
		top: 22px;
		width: 6px;
		height: 6px;
		border-radius: 50%;
		background: var(--accent);
		transition:
			transform 200ms ease,
			background 200ms ease;
	}
	.entry.is-read .junction {
		background: transparent;
		border: 1px solid var(--ink-4);
	}

	.saved-mark {
		position: absolute;
		right: 24px;
		top: 18px;
		font-size: 10px;
		letter-spacing: 0.04em;
		color: var(--accent);
	}

	.title {
		font-family: var(--serif);
		font-size: 17px;
		line-height: 1.3;
		font-weight: 500;
		color: var(--ink);
		margin: 0 0 4px;
		text-wrap: pretty;
	}
	.meta {
		font-family: var(--sans);
		font-size: 12px;
		color: var(--ink-2);
		display: flex;
		align-items: center;
		gap: 10px;
		margin-top: 5px;
	}
	.meta .source {
		color: var(--ink);
		font-weight: 500;
	}
	.meta .ico {
		width: 9px;
		height: 9px;
		border-radius: 2px;
		display: inline-block;
		flex-shrink: 0;
	}
	.meta .sep {
		display: inline-block;
		width: 3px;
		height: 3px;
		background: var(--ink-4);
		border-radius: 50%;
		flex-shrink: 0;
	}
	.meta .rt {
		font-size: 10.5px;
		color: var(--ink-3);
	}

	.summary {
		font-family: var(--serif);
		font-size: 14px;
		line-height: 1.5;
		color: var(--ink-2);
		margin: 4px 0 0;
		display: -webkit-box;
		-webkit-line-clamp: 2;
		line-clamp: 2;
		-webkit-box-orient: vertical;
		overflow: hidden;
		text-wrap: pretty;
	}

	/* Density classes are applied to the .river container by RiverList,
	   so the EntryRow's :global selectors target ancestor classes. */
	:global(.density-compact) .entry {
		padding-top: 10px;
		padding-bottom: 10px;
	}
	:global(.density-compact) .entry .summary {
		display: none;
	}
	:global(.density-compact) .entry .junction {
		top: 17px;
	}
	:global(.density-compact) .entry .saved-mark {
		top: 13px;
	}
	:global(.density-comfortable) .entry {
		padding-top: 16px;
		padding-bottom: 16px;
	}

	/* Mobile overrides cascade from the .is-mobile root applied in
	   +page.svelte. */
	:global(.is-mobile) .entry {
		padding-left: 36px;
		padding-right: 18px;
	}
	:global(.is-mobile) .entry .junction {
		left: 18px;
		top: 22px;
	}
	:global(.is-mobile) .entry .saved-mark {
		right: 18px;
	}
	:global(.is-mobile) .entry .title {
		font-size: 16px;
	}
	:global(.is-mobile) .entry .summary {
		font-size: 13.5px;
	}
</style>
