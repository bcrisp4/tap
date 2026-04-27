<script lang="ts">
	// Desktop sidebar: brand + navigation groups (Reading / Feeds /
	// System). The unread + saved badge counts are driven by live
	// queries — calling `useEntries` with `limit: 1` is sufficient
	// because we only consume `pagination.total` from the response.
	import { useFeeds, useEntries } from '$api/queries';
	import Wordmark from '$brand/Wordmark.svelte';
	import { swatchFor } from './EntryRow.svelte';

	let { active = 'unread' }: { active?: 'unread' | 'all' | 'saved' } = $props();

	const feeds = useFeeds();
	const unread = useEntries({ status: 'unread', limit: 1 });
	const saved = useEntries({ saved: 'true', limit: 1 });
</script>

<aside class="tap-sidebar">
	<div class="brand"><Wordmark /></div>

	<div class="group-title">Reading</div>
	<a href="/" class="nav-item" class:active={active === 'unread'}>
		<span>Unread</span>
		<span class="badge mono">{unread.data?.pagination.total ?? 0}</span>
	</a>
	<a href="/all" class="nav-item" class:active={active === 'all'}>
		<span>All entries</span>
	</a>
	<a href="/saved" class="nav-item" class:active={active === 'saved'}>
		<span>Saved</span>
		<span class="badge mono">{saved.data?.pagination.total ?? 0}</span>
	</a>

	<div class="group-title">Feeds</div>
	{#each feeds.data?.data ?? [] as f (f.id)}
		<a href={'/feed/' + f.id} class="feed-row">
			<span class="ico" style="background: {swatchFor(f.title)}" aria-hidden="true"></span>
			<span class="name">{f.title}</span>
		</a>
	{/each}

	<div class="group-title">System</div>
	<a href="/feeds/add" class="nav-item"><span>Add feed</span></a>
	<a href="/settings" class="nav-item"><span>Settings</span></a>
</aside>

<style>
	.tap-sidebar {
		width: 240px;
		border-right: 1px solid var(--rule);
		padding: 20px 0;
		background: var(--bg);
		overflow-y: auto;
		flex-shrink: 0;
	}
	.brand {
		padding: 0 20px 18px;
		display: flex;
		align-items: center;
		gap: 8px;
	}
	.group-title {
		font-family: var(--mono);
		font-size: 10px;
		letter-spacing: 0.12em;
		text-transform: uppercase;
		color: var(--ink-3);
		padding: 16px 20px 6px;
	}
	.nav-item {
		display: flex;
		align-items: center;
		gap: 10px;
		padding: 6px 20px;
		font-family: var(--sans);
		font-size: 13px;
		color: var(--ink-2);
		text-decoration: none;
		border-left: 2px solid transparent;
	}
	.nav-item:hover {
		color: var(--ink);
	}
	.nav-item.active {
		color: var(--ink);
		border-left-color: var(--accent);
		font-weight: 500;
	}
	.nav-item .badge {
		margin-left: auto;
		font-size: 10.5px;
		color: var(--ink-3);
	}
	.nav-item.active .badge {
		color: var(--accent);
	}
	.feed-row {
		display: flex;
		align-items: center;
		gap: 10px;
		padding: 4px 20px 4px 24px;
		font-family: var(--sans);
		font-size: 12.5px;
		color: var(--ink-2);
		text-decoration: none;
	}
	.feed-row:hover {
		color: var(--ink);
	}
	.feed-row .ico {
		width: 12px;
		height: 12px;
		border-radius: 2px;
		flex-shrink: 0;
		display: inline-block;
	}
	.feed-row .name {
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		flex: 1;
	}
</style>
