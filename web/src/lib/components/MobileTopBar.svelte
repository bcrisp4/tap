<script lang="ts">
	// Compact mobile header: brand + view label + unread count + search
	// affordance (search route ships in Plan 12).
	import Wordmark from '$brand/Wordmark.svelte';

	let { unread = 0 }: { unread?: number } = $props();
</script>

<div class="m-topbar">
	<div class="title">
		<Wordmark />
		<span class="hint">· unread</span>
	</div>
	<div class="right">
		<span class="count mono">{unread}</span>
		<button type="button" class="iconbtn" aria-label="Search">
			<svg
				width="16"
				height="16"
				viewBox="0 0 16 16"
				fill="none"
				stroke="currentColor"
				stroke-width="1.5"
				stroke-linecap="round"
				aria-hidden="true"
			>
				<circle cx="7" cy="7" r="4.5" />
				<path d="m10.5 10.5 3 3" />
			</svg>
		</button>
	</div>
</div>

<style>
	.m-topbar {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 12px 18px 10px;
		border-bottom: 1px solid var(--rule);
		background: var(--bg);
	}
	.title {
		display: flex;
		align-items: baseline;
		gap: 8px;
		font-family: var(--sans);
		font-weight: 600;
		font-size: 16px;
		color: var(--ink);
	}
	.hint {
		font-weight: 400;
		color: var(--ink-3);
		font-size: 13px;
	}
	.right {
		display: flex;
		align-items: center;
		gap: 14px;
	}
	.count {
		font-size: 11px;
		color: var(--ink-3);
	}
	.iconbtn {
		color: var(--ink-2);
		padding: 4px;
		background: transparent;
		border: 0;
		cursor: pointer;
	}
</style>
