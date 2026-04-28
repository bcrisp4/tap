<script lang="ts">
	// /saved — entries the user has explicitly starred. Default
	// ordering is published_at DESC (the established list shape) so
	// the most-recent saved publications appear on top, regardless of
	// when they were saved.
	import { goto } from '$app/navigation';
	import { useFeeds, useEntries } from '$api/queries';
	import { isMobile } from '$lib/breakpoints.svelte';
	import Sidebar from '$lib/components/Sidebar.svelte';
	import TopBar from '$lib/components/TopBar.svelte';
	import RiverList from '$lib/components/RiverList.svelte';
	import MobileTopBar from '$lib/components/MobileTopBar.svelte';
	import MobileTabBar from '$lib/components/MobileTabBar.svelte';

	const feeds = useFeeds();
	// status=all + saved=true: include both read and unread saved
	// entries — once you save something it should keep showing up
	// regardless of read state.
	const saved = useEntries({ status: 'all', saved: 'true', limit: 100 });

	const visible = $derived(saved.data?.data ?? []);
	const total = $derived(saved.data?.pagination.total ?? 0);
	const mobile = $derived(isMobile());
</script>

<svelte:head>
	<title>Saved — tap</title>
</svelte:head>

{#if mobile}
	<div class="tap is-mobile">
		<MobileTopBar label="saved" count={total} />
		<div class="m-river-wrap">
			{#if visible.length === 0 && !saved.isLoading}
				<p class="empty mono">no saved entries · star one with the s key</p>
			{:else}
				<RiverList
					entries={visible}
					feeds={feeds.data?.data ?? []}
					density="default"
					showSummary={true}
					onSelect={(id) => goto('/entry/' + id)}
				/>
			{/if}
		</div>
		<MobileTabBar active="saved" />
	</div>
{:else}
	<div class="tap">
		<Sidebar active="saved" />
		<div class="col">
			<TopBar title="Saved" />
			{#if visible.length === 0 && !saved.isLoading}
				<p class="empty mono">no saved entries · star one with the s key</p>
			{:else}
				<RiverList
					entries={visible}
					feeds={feeds.data?.data ?? []}
					density="default"
					showSummary={true}
					onSelect={(id) => goto('/entry/' + id)}
				/>
			{/if}
		</div>
	</div>
{/if}

<style>
	.tap {
		display: flex;
		height: 100vh;
		background: var(--bg);
		color: var(--ink);
		font-family: var(--serif);
		font-feature-settings: 'kern', 'liga', 'onum';
		-webkit-font-smoothing: antialiased;
		text-rendering: optimizeLegibility;
	}
	.tap.is-mobile {
		flex-direction: column;
	}
	.col {
		flex: 1;
		display: flex;
		flex-direction: column;
		min-width: 0;
	}
	.m-river-wrap {
		flex: 1;
		display: flex;
		flex-direction: column;
		min-height: 0;
		overflow-y: auto;
	}
	.empty {
		padding: 80px 24px;
		text-align: center;
		color: var(--ink-3);
		font-size: 11px;
		letter-spacing: 0.06em;
		text-transform: uppercase;
	}
</style>
