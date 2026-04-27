<script lang="ts">
	// Sticky header for the desktop unread view: title crumb, count,
	// and three icon buttons (search, refresh, mark-all-read). The
	// search button is wired in Plan 12; we render the affordance now.
	let {
		title = 'Unread',
		unread = 0,
		total = 0,
		onMarkAllRead = () => {},
		onRefresh = () => {}
	}: {
		title?: string;
		unread?: number;
		total?: number;
		onMarkAllRead?: () => void;
		onRefresh?: () => void;
	} = $props();
</script>

<div class="tap-topbar">
	<div class="crumb"><b>{title}</b></div>
	<span class="count mono">{unread} of {total}</span>
	<div class="spacer"></div>
	<button type="button" class="icon-btn" title="Search (/)" aria-label="Search">
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
	<button
		type="button"
		class="icon-btn"
		title="Refresh"
		aria-label="Refresh"
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
	<button
		type="button"
		class="icon-btn"
		title="Mark all read (Shift+M)"
		aria-label="Mark all read"
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
	.icon-btn:hover {
		background: var(--bg-soft);
		color: var(--ink);
	}
</style>
