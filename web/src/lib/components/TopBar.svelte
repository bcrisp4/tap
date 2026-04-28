<script lang="ts">
	// Sticky header. The unread view needs the full chrome (counter +
	// refresh + mark-all-read); other routes (Saved, History, Settings,
	// Feeds, Search) only want the title crumb. To avoid forking into a
	// second component, the counter and the two action buttons each
	// only render when the route opts in:
	//
	//   - `unread`/`total` are optional. The counter renders only when
	//     `total` is supplied (i.e. the page is making a meaningful
	//     "n of m" claim). On `/saved`/`/history` we deliberately skip
	//     it so the header doesn't read as an unread counter.
	//   - `onRefresh`/`onMarkAllRead` are optional. The corresponding
	//     button only renders when the route passes a real handler;
	//     otherwise the affordance disappears entirely (no clickable
	//     no-op buttons).
	//
	// Plan 15 wires Refresh for real (Plan 10's stub TODO) and adds a
	// `refreshing` flag so the parent can disable the button + show a
	// loading state while the fan-out POST /feeds/[id]/refresh calls
	// are in flight.
	//
	// The Search affordance still renders disabled — it advertises a
	// future capability without pretending to work today.
	let {
		title = 'Unread',
		unread,
		total,
		onMarkAllRead,
		onRefresh,
		refreshing = false
	}: {
		title?: string;
		unread?: number;
		total?: number;
		onMarkAllRead?: () => void;
		onRefresh?: () => void;
		refreshing?: boolean;
	} = $props();
</script>

<div class="tap-topbar">
	<div class="crumb"><b>{title}</b></div>
	{#if total !== undefined}
		<span class="count mono">{unread ?? 0} of {total}</span>
	{/if}
	<div class="spacer"></div>
	<button
		type="button"
		class="icon-btn"
		title="Search (lands in Plan 14)"
		aria-label="Search"
		disabled
		aria-disabled="true"
	>
		<svg
			width="14"
			height="14"
			viewBox="0 0 16 16"
			fill="none"
			stroke="currentColor"
			stroke-width="1.4"
			stroke-linecap="round"
			aria-hidden="true"
		>
			<circle cx="7" cy="7" r="4.5" />
			<path d="m10.5 10.5 3 3" />
		</svg>
	</button>
	{#if onRefresh}
		<button
			type="button"
			class="icon-btn"
			class:is-spinning={refreshing}
			title="Refresh"
			aria-label="Refresh"
			disabled={refreshing}
			data-testid="refresh-button"
			onclick={onRefresh}
		>
			<svg
				width="14"
				height="14"
				viewBox="0 0 16 16"
				fill="none"
				stroke="currentColor"
				stroke-width="1.4"
				stroke-linecap="round"
				stroke-linejoin="round"
				aria-hidden="true"
			>
				<path d="M14 8a6 6 0 1 1-1.76-4.24" />
				<path d="M14 2.5V6h-3.5" />
			</svg>
		</button>
	{/if}
	{#if onMarkAllRead}
		<button
			type="button"
			class="icon-btn"
			title="Mark all read"
			aria-label="Mark all read"
			data-testid="mark-all-read-button"
			onclick={onMarkAllRead}
		>
			<svg
				width="14"
				height="14"
				viewBox="0 0 16 16"
				fill="none"
				stroke="currentColor"
				stroke-width="1.4"
				stroke-linecap="round"
				stroke-linejoin="round"
				aria-hidden="true"
			>
				<path d="m3 8 3.5 3.5L13 5" />
			</svg>
		</button>
	{/if}
</div>

<style>
	.tap-topbar {
		display: flex;
		align-items: center;
		gap: 12px;
		padding: 14px 24px;
		border-bottom: 1px solid var(--rule);
		background: var(--bg);
		position: sticky;
		top: 0;
		z-index: 5;
	}
	.crumb {
		font-family: var(--sans);
		font-size: 13px;
		color: var(--ink-2);
	}
	.crumb b {
		color: var(--ink);
		font-weight: 600;
	}
	.count {
		font-size: 11px;
		color: var(--ink-3);
		letter-spacing: 0.02em;
	}
	.spacer {
		flex: 1;
	}
	.icon-btn {
		width: 28px;
		height: 28px;
		display: inline-grid;
		place-items: center;
		color: var(--ink-2);
		border-radius: 4px;
		background: transparent;
		border: 0;
		padding: 0;
		cursor: pointer;
	}
	.icon-btn:disabled {
		color: var(--ink-4);
		cursor: not-allowed;
	}
	@media (hover: hover) {
		.icon-btn:hover {
			background: var(--bg-soft);
			color: var(--ink);
		}
		.icon-btn:disabled:hover {
			background: transparent;
			color: var(--ink-4);
		}
	}
	.icon-btn.is-spinning svg {
		animation: tb-spin 0.9s linear infinite;
	}
	.icon-btn.is-spinning:disabled {
		color: var(--accent);
		cursor: progress;
	}
	@keyframes tb-spin {
		from {
			transform: rotate(0deg);
		}
		to {
			transform: rotate(360deg);
		}
	}
</style>
