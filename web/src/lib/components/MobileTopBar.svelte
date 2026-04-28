<script lang="ts">
	import Wordmark from '$brand/Wordmark.svelte';

	let {
		label = 'unread',
		count = null,
		showSearch = true
	}: {
		label?: string;
		count?: number | null;
		showSearch?: boolean;
	} = $props();
</script>

<div class="m-topbar">
	<div class="title">
		<Wordmark />
		<span class="hint">· {label}</span>
	</div>
	<div class="right">
		{#if count !== null}
			<span class="count mono">{count}</span>
		{/if}
		{#if showSearch}
			<a class="iconbtn" href="/search" aria-label="Search">
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
			</a>
		{/if}
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
		display: inline-flex;
		align-items: center;
		justify-content: center;
		color: var(--ink-2);
		padding: 4px;
		background: transparent;
		border: 0;
		cursor: pointer;
		text-decoration: none;
	}
	.iconbtn:hover {
		color: var(--ink);
	}
</style>
