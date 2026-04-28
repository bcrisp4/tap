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
	import { inputMode } from '$lib/inputmode.svelte';

	let {
		entry,
		feed,
		selected = false,
		showSummary = true,
		multiSelect = false,
		multiSelected = false,
		onclick = (_ev: MouseEvent) => {},
		onToggleRead = (_id: number, _read: boolean) => {},
		onToggleSelect = (_id: number, _ev: MouseEvent) => {}
	}: {
		entry: Entry;
		feed?: Feed;
		selected?: boolean;
		showSummary?: boolean;
		multiSelect?: boolean;
		multiSelected?: boolean;
		onclick?: (e: MouseEvent) => void;
		onToggleRead?: (id: number, read: boolean) => void;
		onToggleSelect?: (id: number, ev: MouseEvent) => void;
	} = $props();

	const ago = $derived(formatAgo(entry.published_at ?? entry.created_at));
	const swatchColor = $derived(swatchFor(feed?.title ?? feed?.feed_url ?? 'tap'));
	// Keyboard-selection highlight is only meaningful when the user is
	// actually driving with the keyboard. On mouse / touch the row that
	// happens to be `selectedId` shouldn't paint highlighted (the user
	// has no way of knowing why one row is shaded).
	const showKeyboardHighlight = $derived(selected && inputMode.mode === 'keyboard');

	function onReadDotClick(ev: MouseEvent) {
		// Stop the row click; the read-dot has its own action and we
		// don't want a click-to-open hijack.
		ev.stopPropagation();
		ev.preventDefault();
		onToggleRead(entry.id, !entry.read);
	}

	function onSelectBoxClick(ev: MouseEvent) {
		ev.stopPropagation();
		ev.preventDefault();
		onToggleSelect(entry.id, ev);
	}

	function onLinkClick(ev: MouseEvent) {
		// The parent owns navigation (RiverList → +page.svelte). Honour
		// the same modifier-key contract the parent expects: shift /
		// meta / ctrl get routed to multi-select toggle. The default
		// path lets the parent's onclick handle navigation via the
		// SPA router, so we preventDefault to suppress the link's
		// native full-page-navigation.
		ev.preventDefault();
		onclick(ev);
	}
</script>

<article
	class="entry"
	class:is-read={entry.read}
	class:is-saved={entry.saved}
	class:is-selected={showKeyboardHighlight}
	class:is-multi={multiSelect}
	class:is-multi-selected={multiSelected}
>
	<!--
		Per-row affordance. Real <button> with native keyboard /
		focus / a11y semantics. Sits visually in the row gutter via
		position:absolute; the stretched <a> link below covers the
		rest of the row and is the click-to-open target.
	-->
	{#if multiSelect}
		<button
			type="button"
			class="select-box"
			aria-label={multiSelected ? 'Deselect entry' : 'Select entry'}
			aria-pressed={multiSelected}
			onclick={onSelectBoxClick}
		>
			<span class="check" aria-hidden="true">{multiSelected ? '✓' : ''}</span>
		</button>
	{:else}
		<button
			type="button"
			class="read-dot"
			aria-label={entry.read ? 'Mark unread' : 'Mark read'}
			aria-pressed={!entry.read}
			data-testid="row-read-toggle"
			onclick={onReadDotClick}
		>
			<span class="dot" aria-hidden="true"></span>
		</button>
	{/if}
	{#if entry.saved}
		<span class="saved-mark mono">SAVED</span>
	{/if}
	<!--
		Stretched link covers the row's clickable area. <a href> is
		natively keyboard-focusable (Tab) and Enter activates it. The
		parent owns SPA navigation, so we preventDefault and forward
		the click to the parent's handler. The href is still set for
		middle-click / cmd-click / "open in new tab" behaviour and for
		assistive tech which announces the URL.
	-->
	<a
		class="hit"
		href={'/entry/' + entry.id}
		aria-label={'Open entry: ' + entry.title}
		onclick={onLinkClick}
	></a>
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
</article>

<style>
	.entry {
		position: relative;
		display: block;
		text-align: left;
		width: 100%;
		padding: 14px 24px 14px 40px;
		border-bottom: 1px solid var(--rule);
		background: transparent;
		transition: background 120ms ease;
		font-family: inherit;
		color: inherit;
		box-sizing: border-box;
	}
	.entry:hover {
		background: var(--bg-soft);
	}
	.entry.is-selected {
		background: var(--accent-soft);
	}
	.entry.is-multi-selected {
		background: var(--accent-soft);
	}
	.entry.is-read .title {
		color: var(--ink-3);
		font-weight: 400;
	}
	.entry.is-read .meta {
		color: var(--ink-3);
	}

	/* Stretched link: covers the row, sits BEHIND the action buttons
	   (lower z-index) so the buttons get pointer events first.
	   Visually invisible — the row's text shows through. */
	.hit {
		position: absolute;
		inset: 0;
		z-index: 0;
		text-indent: -9999px;
		overflow: hidden;
		cursor: pointer;
	}
	.hit:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: -2px;
	}

	/* Per-row mark-read button. Filled dot = unread, hollow ring =
	   read. Hidden on mobile — Plan 18 swaps in swipe gestures. */
	.read-dot {
		position: absolute;
		left: 16px;
		top: 16px;
		width: 18px;
		height: 18px;
		z-index: 1;
		display: inline-grid;
		place-items: center;
		background: transparent;
		border: 0;
		border-radius: 50%;
		padding: 0;
		cursor: pointer;
		color: inherit;
	}
	.read-dot:hover {
		background: var(--bg-soft);
	}
	.read-dot .dot {
		width: 6px;
		height: 6px;
		border-radius: 50%;
		background: var(--accent);
		transition:
			transform 200ms ease,
			background 200ms ease;
		box-sizing: border-box;
	}
	.entry.is-read .read-dot .dot {
		background: transparent;
		border: 1px solid var(--ink-4);
	}

	/* Multi-select checkbox replaces the read dot when the river is in
	   multi-select mode. */
	.select-box {
		position: absolute;
		left: 14px;
		top: 14px;
		width: 18px;
		height: 18px;
		z-index: 1;
		display: inline-grid;
		place-items: center;
		background: var(--bg);
		border: 1px solid var(--ink-4);
		border-radius: 3px;
		padding: 0;
		cursor: pointer;
		color: inherit;
	}
	.entry.is-multi-selected .select-box {
		background: var(--accent);
		border-color: var(--accent);
		color: var(--bg);
	}
	.select-box .check {
		font-size: 12px;
		line-height: 1;
		font-family: var(--sans);
	}

	.saved-mark {
		position: absolute;
		right: 24px;
		top: 18px;
		font-size: 10px;
		letter-spacing: 0.04em;
		color: var(--accent);
		z-index: 1;
		pointer-events: none;
	}

	.title {
		font-family: var(--serif);
		font-size: 17px;
		line-height: 1.3;
		font-weight: 500;
		color: var(--ink);
		margin: 0 0 4px;
		text-wrap: pretty;
		position: relative;
		pointer-events: none;
	}
	.meta {
		font-family: var(--sans);
		font-size: 12px;
		color: var(--ink-2);
		display: flex;
		align-items: center;
		gap: 10px;
		margin-top: 5px;
		position: relative;
		pointer-events: none;
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
		position: relative;
		pointer-events: none;
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
	:global(.density-compact) .entry .read-dot {
		top: 11px;
	}
	:global(.density-compact) .entry .select-box {
		top: 9px;
	}
	:global(.density-compact) .entry .saved-mark {
		top: 13px;
	}
	:global(.density-comfortable) .entry {
		padding-top: 16px;
		padding-bottom: 16px;
	}

	/* Mobile overrides cascade from the .is-mobile root applied in
	   +page.svelte. Plan 18 will add swipe gestures; for now the
	   per-row read dot is hidden on mobile to keep the row tappable. */
	:global(.is-mobile) .entry {
		padding-left: 36px;
		padding-right: 18px;
	}
	:global(.is-mobile) .entry .read-dot {
		display: none;
	}
	:global(.is-mobile) .entry.is-multi .select-box {
		left: 14px;
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
