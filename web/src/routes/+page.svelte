<script lang="ts">
	// The unread "river" — the home view of Tap.
	//
	// Two layouts switch on a `(max-width: 720px)` matchMedia query:
	//   - desktop: sidebar + top bar + poll strip + list + hints footer
	//   - mobile : top bar + list + bottom tab bar
	//
	// Selection + keyboard nav are owned here so all the chrome
	// components stay pure-presentation.
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { useQueryClient } from '@tanstack/svelte-query';
	import { useFeeds, useEntries, useToggleRead, useToggleSaved, keys } from '$api/queries';
	import { putJSON } from '$api/client';
	import Sidebar from '$lib/components/Sidebar.svelte';
	import TopBar from '$lib/components/TopBar.svelte';
	import PollStrip from '$lib/components/PollStrip.svelte';
	import RiverList from '$lib/components/RiverList.svelte';
	import MobileTopBar from '$lib/components/MobileTopBar.svelte';
	import MobileTabBar from '$lib/components/MobileTabBar.svelte';
	import { bindKeyboard } from '$lib/keyboard.svelte';

	const qc = useQueryClient();
	const feeds = useFeeds();
	const entries = useEntries({ status: 'unread' });
	const toggleRead = useToggleRead();
	const toggleSaved = useToggleSaved();

	// `userSelectedId` is the explicit user choice (null until they
	// click or press j/k). `selectedId` is what the UI actually
	// displays — it falls through to the first visible entry while
	// the user hasn't picked one OR when the previously-picked entry
	// no longer matches anything in the current list (e.g. they just
	// pressed `m` and the entry dropped off the unread filter). This
	// split keeps the default-pick rule pure-derived rather than
	// living inside an `$effect` that writes state.
	let userSelectedId = $state<number | null>(null);
	const visible = $derived(entries.data?.data ?? []);
	const total = $derived(entries.data?.pagination.total ?? 0);
	const selectedId = $derived(
		userSelectedId !== null && visible.some((e) => e.id === userSelectedId)
			? userSelectedId
			: (visible[0]?.id ?? null)
	);

	function indexOfSelected(): number {
		return visible.findIndex((e) => e.id === selectedId);
	}

	bindKeyboard({
		onNext: () => {
			const i = indexOfSelected();
			if (i >= 0 && i < visible.length - 1) userSelectedId = visible[i + 1].id;
		},
		onPrev: () => {
			const i = indexOfSelected();
			if (i > 0) userSelectedId = visible[i - 1].id;
		},
		onToggleRead: () => {
			const cur = visible.find((e) => e.id === selectedId);
			if (cur) toggleRead.mutate({ id: cur.id, read: !cur.read });
		},
		onToggleSaved: () => {
			const cur = visible.find((e) => e.id === selectedId);
			if (cur) toggleSaved.mutate({ id: cur.id, saved: !cur.saved });
		},
		onOpen: () => {
			if (selectedId !== null) void goto('/entry/' + selectedId);
		},
		onViewOriginal: () => {
			const cur = visible.find((e) => e.id === selectedId);
			if (cur?.url) window.open(cur.url, '_blank', 'noopener,noreferrer');
		}
	});

	async function markAllRead() {
		// Bulk mark: design.md §6 specifies PUT /entries/read.
		// Empty body = "mark all unread entries (across all feeds) read".
		// Errors are caught locally so they don't surface as unhandled
		// rejections; on success we invalidate the entries cache so the
		// river refetches and the now-read entries drop off.
		try {
			await putJSON('/entries/read', {});
			await qc.invalidateQueries({ queryKey: keys.entriesAll() });
		} catch (err) {
			console.error('mark-all-read failed', err);
		}
	}

	function refresh() {
		// Manual refresh wiring lands in Plan 12 (pull-to-refresh + the
		// /feeds/:id/refresh trigger). Today the poller drives state.
	}

	let isMobile = $state(false);
	onMount(() => {
		const mq = window.matchMedia('(max-width: 720px)');
		const apply = () => {
			isMobile = mq.matches;
		};
		apply();
		mq.addEventListener('change', apply);
		return () => mq.removeEventListener('change', apply);
	});
</script>

{#if isMobile}
	<div class="tap is-mobile">
		<MobileTopBar label="unread" count={total} />
		<div class="m-river-wrap">
			<RiverList
				entries={visible}
				feeds={feeds.data?.data ?? []}
				{selectedId}
				density="default"
				showSummary={true}
				onSelect={(id) => goto('/entry/' + id)}
			/>
		</div>
		<MobileTabBar active="unread" />
	</div>
{:else}
	<div class="tap">
		<Sidebar active="unread" />
		<div class="col">
			<TopBar
				title="Unread"
				unread={total}
				total={total}
				onMarkAllRead={markAllRead}
				onRefresh={refresh}
			/>
			<PollStrip />
			<RiverList
				entries={visible}
				feeds={feeds.data?.data ?? []}
				{selectedId}
				density="default"
				showSummary={true}
				onSelect={(id) => (userSelectedId = id)}
			/>
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
	}
	/* Webkit scrollbar treatment for the entire app surface — ported
	   from docs/ui_design_handoff/styles.css :: .tap *::-webkit-* */
	.tap :global(*::-webkit-scrollbar) {
		width: 8px;
		height: 8px;
	}
	.tap :global(*::-webkit-scrollbar-thumb) {
		background: var(--ink-4);
		border-radius: 4px;
	}
	.tap :global(*::-webkit-scrollbar-track) {
		background: transparent;
	}
	/* Match the handoff's accent focus ring. */
	.tap :global(*:focus-visible) {
		outline: 2px solid var(--accent);
		outline-offset: 2px;
	}
</style>
