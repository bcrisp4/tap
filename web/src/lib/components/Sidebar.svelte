<script lang="ts">
	// Desktop sidebar: brand + Reading nav + Feeds list + footer icon
	// strip. The unread + saved badge counts are driven by live queries
	// — calling `useEntries` with `limit: 1` is sufficient because we
	// only consume `pagination.total` from the response.
	//
	// Settings, theme switcher, and hotkeys help live in `SidebarFooter`
	// at the bottom — they're secondary chrome controls, not primary
	// nav. "Add feed" lives as a small `+` next to the FEEDS heading.
	import { useFeeds, useEntries } from '$api/queries';
	import Wordmark from '$brand/Wordmark.svelte';
	import FeedIcon from './FeedIcon.svelte';
	import SidebarFooter from './SidebarFooter.svelte';
	import { AlertTriangle } from 'lucide-svelte';

	let { active }: { active?: 'unread' | 'history' | 'saved' } = $props();

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
	<a href="/history" class="nav-item" class:active={active === 'history'}>
		<span>History</span>
	</a>
	<a href="/saved" class="nav-item" class:active={active === 'saved'}>
		<span>Saved</span>
		<span class="badge mono">{saved.data?.pagination.total ?? 0}</span>
	</a>

	<div class="group-title group-title-row">
		<span>Feeds</span>
		<a class="add-feed" href="/feeds/add" aria-label="Add feed" title="Add feed">+</a>
	</div>
	{#each feeds.data?.data ?? [] as f (f.id)}
		<a href={'/feeds/' + f.id} class="feed-row">
			<FeedIcon feed={f} />
			<span class="name">{f.title}</span>
			{#if f.error_count > 0}
				<AlertTriangle
					class="feed-warn"
					size="12"
					aria-hidden="false"
					aria-label={`Feed has errors: ${f.last_error ?? ''}`}
					title={f.last_error ?? ''}
				/>
			{/if}
		</a>
	{/each}

	<div class="footer-spacer"></div>
	<SidebarFooter />
</aside>

<style>
	.tap-sidebar {
		width: 240px;
		border-right: 1px solid var(--rule);
		padding: 20px 0 0;
		background: var(--bg);
		overflow-y: auto;
		flex-shrink: 0;
		display: flex;
		flex-direction: column;
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
	.group-title-row {
		display: flex;
		align-items: center;
		justify-content: space-between;
	}
	.add-feed {
		font-family: var(--mono);
		font-size: 14px;
		line-height: 1;
		color: var(--ink-3);
		text-decoration: none;
		padding: 0 6px;
		border-radius: 3px;
	}
	@media (hover: hover) {
		.add-feed:hover {
			color: var(--ink);
			background: var(--surface);
		}
	}
	.footer-spacer {
		flex: 1;
		min-height: 12px;
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
	@media (hover: hover) {
		.nav-item:hover {
			color: var(--ink);
		}
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
	@media (hover: hover) {
		.feed-row:hover {
			color: var(--ink);
		}
	}
	.feed-row .name {
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		flex: 1;
	}
	.feed-row :global(.feed-warn) {
		color: var(--accent-warn, #c33);
		flex-shrink: 0;
		margin-left: 4px;
	}
</style>
