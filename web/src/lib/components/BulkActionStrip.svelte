<script lang="ts">
	// Sticky strip that appears at the top of the unread river when the
	// user enters multi-select mode (Plan 15). Shows the count and the
	// three actions: Mark read · Mark unread · Clear. The component is
	// pure-presentation; the parent owns the selection set and the
	// bulk mutation hook.

	let {
		count,
		onMarkRead,
		onMarkUnread,
		onClear
	}: {
		count: number;
		onMarkRead: () => void;
		onMarkUnread: () => void;
		onClear: () => void;
	} = $props();
</script>

<div class="bulk-strip" data-testid="bulk-action-strip">
	<span class="count mono">{count} selected</span>
	<div class="spacer"></div>
	<button type="button" class="action" onclick={onMarkRead} data-testid="bulk-mark-read">
		Mark read
	</button>
	<button type="button" class="action" onclick={onMarkUnread} data-testid="bulk-mark-unread">
		Mark unread
	</button>
	<button type="button" class="action ghost" onclick={onClear} data-testid="bulk-clear">
		Clear
	</button>
</div>

<style>
	.bulk-strip {
		display: flex;
		align-items: center;
		gap: 12px;
		padding: 8px 24px;
		border-bottom: 1px solid var(--rule);
		background: var(--accent-soft);
		position: sticky;
		top: 50px;
		z-index: 4;
	}
	.count {
		font-size: 11px;
		letter-spacing: 0.04em;
		color: var(--ink);
		font-weight: 600;
	}
	.spacer {
		flex: 1;
	}
	.action {
		font-family: var(--sans);
		font-size: 12px;
		padding: 4px 10px;
		border-radius: 3px;
		border: 1px solid var(--rule);
		background: var(--bg);
		color: var(--ink);
		cursor: pointer;
	}
	.action.ghost {
		background: transparent;
		border-color: transparent;
		color: var(--ink-2);
	}
	@media (hover: hover) {
		.action:hover {
			background: var(--bg-soft);
		}
		.action.ghost:hover {
			color: var(--ink);
		}
	}
</style>
