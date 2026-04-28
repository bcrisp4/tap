<script lang="ts">
	// /history — entries you've already read, newest-read first.
	//
	// The view is intentionally passive: no selection, no keyboard
	// shortcuts (those are owned by the unread river and the reader),
	// just a chronologically-by-read time list. Tapping an entry
	// navigates to the reader, the same pattern as the rest of Tap.
	import { goto } from '$app/navigation';
	import { useFeeds, useHistory } from '$api/queries';
	import { isMobile } from '$lib/breakpoints.svelte';
	import Sidebar from '$lib/components/Sidebar.svelte';
	import TopBar from '$lib/components/TopBar.svelte';
	import RiverList from '$lib/components/RiverList.svelte';
	import MobileTopBar from '$lib/components/MobileTopBar.svelte';
	import MobileTabBar from '$lib/components/MobileTabBar.svelte';

	const feeds = useFeeds();
	const history = useHistory({ limit: 100 });

	const visible = $derived(history.data?.data ?? []);
	const total = $derived(history.data?.pagination.total ?? 0);
	const mobile = $derived(isMobile());
</script>

<svelte:head>
	<title>History — tap</title>
</svelte:head>

{#if mobile}
	<div class="tap is-mobile">
		<MobileTopBar label="history" count={total} />
		<div class="m-river-wrap">
			{#if visible.length === 0 && !history.isLoading}
				<p class="empty mono">no history yet · entries you've read appear here</p>
			{:else}
				<RiverList
					entries={visible}
					feeds={feeds.data?.data ?? []}
					density="default"
					showSummary={true}
					dimRead={false}
					onSelect={(id) => goto('/entry/' + id)}
				/>
			{/if}
		</div>
		<MobileTabBar active="history" />
	</div>
{:else}
	<div class="tap">
		<Sidebar active="history" />
		<div class="col">
			<TopBar title="History" />
			{#if visible.length === 0 && !history.isLoading}
				<p class="empty mono">no history yet · entries you've read appear here</p>
			{:else}
				<RiverList
					entries={visible}
					feeds={feeds.data?.data ?? []}
					density="default"
					showSummary={true}
					dimRead={false}
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
