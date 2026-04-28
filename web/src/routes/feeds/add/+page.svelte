<script lang="ts">
	// /feeds/add — discover and subscribe to a new feed.
	import { goto } from '$app/navigation';
	import { useDiscoverFeed, useSubscribeFeed } from '$api/queries';
	import { ApiError } from '$api/client';
	import type { DiscoverCandidate } from '$api/types';
	import { isMobile } from '$lib/breakpoints.svelte';
	import Sidebar from '$lib/components/Sidebar.svelte';
	import TopBar from '$lib/components/TopBar.svelte';
	import FeedForm from '$lib/components/FeedForm.svelte';
	import MobileTopBar from '$lib/components/MobileTopBar.svelte';
	import MobileTabBar from '$lib/components/MobileTabBar.svelte';

	const subscribe = useSubscribeFeed();
	const discover = useDiscoverFeed();

	// Locally-tracked outcome state — TanStack mutations expose data,
	// but for the inline UX here we only need the candidates list and
	// any human-readable error string.
	let candidates = $state<DiscoverCandidate[]>([]);
	let discoverError = $state<string | null>(null);
	let subscribeError = $state<string | null>(null);

	function handleDiscover(url: string) {
		discoverError = null;
		discover.mutate(url, {
			onSuccess: (res) => {
				candidates = res.candidates;
				if (res.candidates.length === 0) {
					discoverError = 'no feeds found at that URL';
				}
			},
			onError: (err) => {
				candidates = [];
				discoverError = err instanceof ApiError ? err.message : 'discovery failed';
			}
		});
	}

	function handleSubscribe(body: { feed_url: string; title?: string }) {
		subscribeError = null;
		subscribe.mutate(body, {
			onSuccess: (res) => goto(`/feeds/${res.id}`),
			onError: (err) => {
				if (err instanceof ApiError) {
					subscribeError = err.message;
				} else {
					subscribeError = 'subscription failed';
				}
			}
		});
	}

	const mobile = $derived(isMobile());
</script>

<svelte:head>
	<title>Add feed — tap</title>
</svelte:head>

{#snippet content()}
	<div class="add-wrap">
		<header class="head">
			<h1>Subscribe to a new feed</h1>
			<p class="hint">
				Paste a feed URL or a homepage — Discover will find any RSS, Atom, or JSON feeds it
				links to.
			</p>
		</header>
		<FeedForm
			mode="add"
			busy={subscribe.isPending}
			discoverPending={discover.isPending}
			{candidates}
			{discoverError}
			errorMessage={subscribeError}
			onDiscover={handleDiscover}
			onSubscribe={handleSubscribe}
		/>
	</div>
{/snippet}

{#if mobile}
	<div class="tap is-mobile">
		<MobileTopBar label="add feed" />
		<div class="m-wrap">{@render content()}</div>
		<MobileTabBar />
	</div>
{:else}
	<div class="tap">
		<Sidebar />
		<div class="col">
			<TopBar title="Add feed" />
			<div class="scroll">{@render content()}</div>
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
	.scroll,
	.m-wrap {
		flex: 1;
		overflow-y: auto;
		min-height: 0;
	}
	.add-wrap {
		max-width: 640px;
		margin: 0 auto;
		padding: 36px 28px 80px;
	}
	.head {
		margin-bottom: 22px;
	}
	.head h1 {
		font-family: var(--serif);
		font-size: 26px;
		font-weight: 500;
		letter-spacing: -0.01em;
		margin: 0 0 6px;
		color: var(--ink);
	}
	.head .hint {
		font-family: var(--sans);
		font-size: 13px;
		line-height: 1.5;
		color: var(--ink-3);
		margin: 0;
	}
</style>
