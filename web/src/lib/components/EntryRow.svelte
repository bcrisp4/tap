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
	import { createSwipe, type SwipeEvent } from '$lib/swipe.svelte';

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

	// Tracks which entry-id was acted on by the most-recent pointerup so
	// the synthesized click that follows a touch tap can be suppressed
	// without affecting genuine mouse clicks (which deliver pointerup +
	// click on the same target). On mobile the synthesized click after
	// a touch tap can land on the stretched <a> sibling instead of the
	// button, navigating into the reader instead of toggling read —
	// pointerup + stopPropagation guarantees the toggle wins.
	let suppressNextDotClick = false;
	let suppressNextSelectClick = false;

	function onReadDotPointerUp(ev: PointerEvent) {
		// Only handle primary-button pointer ups. Mouse right-click and
		// middle-click should fall through to the browser's defaults.
		if (ev.button !== 0) return;
		ev.stopPropagation();
		ev.preventDefault();
		suppressNextDotClick = true;
		onToggleRead(entry.id, !entry.read);
	}

	function onReadDotClick(ev: MouseEvent) {
		// pointerup already handled the toggle; the click is the
		// browser's compatibility echo. Swallow it so it doesn't
		// double-fire (or, worse, retarget at the underlying anchor
		// after a touch tap).
		if (suppressNextDotClick) {
			suppressNextDotClick = false;
			ev.stopPropagation();
			ev.preventDefault();
			return;
		}
		// Fallback for environments that didn't dispatch pointerup —
		// keyboard-activated clicks (Space / Enter) come in as a click
		// without a preceding pointerup.
		ev.stopPropagation();
		ev.preventDefault();
		onToggleRead(entry.id, !entry.read);
	}

	function onSelectBoxPointerUp(ev: PointerEvent) {
		if (ev.button !== 0) return;
		ev.stopPropagation();
		ev.preventDefault();
		suppressNextSelectClick = true;
		onToggleSelect(entry.id, ev as unknown as MouseEvent);
	}

	function onSelectBoxClick(ev: MouseEvent) {
		if (suppressNextSelectClick) {
			suppressNextSelectClick = false;
			ev.stopPropagation();
			ev.preventDefault();
			return;
		}
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

	// Plan 18 / T5: swipe-to-mark-read on touch devices. Right-swipe →
	// mark read (matches "completed" semantics, fits the visual flow
	// of dragging the row off-screen toward the right). Left-swipe →
	// mark unread (the symmetric inverse).
	//
	// We render the swipeDx as a translateX so the row visually
	// follows the finger. After the gesture ends the row springs back
	// because swipeDx resets to 0; the cache flip after the mutate
	// re-renders with the new read state.
	const swipe = createSwipe({
		threshold: 60,
		onSwipe: (ev: SwipeEvent) => {
			if (ev.direction === 'right') {
				onToggleRead(entry.id, true);
			} else {
				onToggleRead(entry.id, false);
			}
		}
	});
</script>

<article
	class="entry"
	class:is-read={entry.read}
	class:is-saved={entry.saved}
	class:is-selected={showKeyboardHighlight}
	class:is-multi={multiSelect}
	class:is-multi-selected={multiSelected}
	class:is-swiping={swipe.swipeDx !== 0}
	style:transform={swipe.swipeDx !== 0 ? `translateX(${swipe.swipeDx}px)` : undefined}
	ontouchstart={swipe.onTouchStart}
	ontouchmove={swipe.onTouchMove}
	ontouchend={swipe.onTouchEnd}
	ontouchcancel={swipe.onTouchCancel}
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
			onpointerup={onSelectBoxPointerUp}
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
			onpointerup={onReadDotPointerUp}
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
		transition:
			background 120ms ease,
			transform 220ms cubic-bezier(0.2, 0.8, 0.4, 1);
		font-family: inherit;
		color: inherit;
		box-sizing: border-box;
	}
	/* While the finger is dragging, suppress the spring-back transition
	   so the row tracks the finger 1:1; the transition kicks back in
	   for the release animation. */
	.entry.is-swiping {
		transition:
			background 120ms ease,
			transform 0ms;
	}
	@media (hover: hover) {
		.entry:hover {
			background: var(--bg-soft);
		}
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
	   read. The button is sized 44x44 (iOS HIG floor) with the dot
	   visually centered via inline-grid; the visible dot itself stays
	   small (.dot, 6px). The 3px offset keeps the dot's visual center
	   at the same (25,25) point Plan 15 designed for the 18px button.
	   `touch-action: manipulation` removes the 300ms tap-delay on
	   touch devices. */
	.read-dot {
		position: absolute;
		left: 3px;
		top: 3px;
		min-width: 44px;
		min-height: 44px;
		width: 44px;
		height: 44px;
		z-index: 1;
		display: inline-grid;
		place-items: center;
		background: transparent;
		border: 0;
		border-radius: 50%;
		padding: 0;
		cursor: pointer;
		color: inherit;
		touch-action: manipulation;
	}
	@media (hover: hover) {
		.read-dot:hover {
			background: var(--bg-soft);
		}
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
	   multi-select mode. The visible 18×18 box is centered inside a
	   44×44 hit area (iOS HIG floor) using a ::before pseudo-element
	   so the visible affordance keeps its compact look while the tap
	   target meets the minimum. */
	.select-box {
		position: absolute;
		left: 0;
		top: 0;
		min-width: 44px;
		min-height: 44px;
		width: 44px;
		height: 44px;
		z-index: 1;
		display: inline-grid;
		place-items: center;
		background: transparent;
		border: 0;
		padding: 0;
		cursor: pointer;
		color: inherit;
		touch-action: manipulation;
	}
	.select-box::before {
		content: '';
		display: block;
		width: 18px;
		height: 18px;
		background: var(--bg);
		border: 1px solid var(--ink-4);
		border-radius: 3px;
		grid-area: 1 / 1;
	}
	.select-box .check {
		grid-area: 1 / 1;
		z-index: 1;
		font-size: 12px;
		line-height: 1;
		font-family: var(--sans);
	}
	.entry.is-multi-selected .select-box::before {
		background: var(--accent);
		border-color: var(--accent);
	}
	.entry.is-multi-selected .select-box {
		color: var(--bg);
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
	:global(.density-compact) .entry .saved-mark {
		top: 13px;
	}
	:global(.density-comfortable) .entry {
		padding-top: 16px;
		padding-bottom: 16px;
	}

	/* Mobile overrides cascade from the .is-mobile root applied in
	   +page.svelte. The read-dot stays visible on mobile (Plan 18 T3 +
	   T4): T3 fixes its tap-vs-open conflict; T4 sizes it ≥44×44 via
	   the dedicated `.read-dot` rules below so the underlying anchor
	   never claims the tap. The swipe gesture (T5) is the secondary
	   affordance, not the only one. */
	:global(.is-mobile) .entry {
		padding-left: 44px;
		padding-right: 18px;
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
