<script lang="ts">
	// The unread "river" — the home view of Tap.
	//
	// Two layouts switch on a `(max-width: 720px)` matchMedia query:
	//   - desktop: sidebar + top bar + list (+ bulk strip when active)
	//   - mobile : top bar + list + bottom tab bar
	//
	// Plan 15 reshapes the desktop chrome:
	//   - Top status row (PollStrip) and pulsing-blue dot are gone.
	//   - OfflineIndicator replaces them, only visible when offline.
	//   - KeyboardHints footer is gone (Plan 17 ships the modal).
	//   - Click-to-open works on desktop; keyboard nav still works.
	//   - Per-row mark-read button + multi-select + bulk read.
	//   - Mark All Read shows a ConfirmModal.
	//   - Refresh button is wired to the real /feeds/[id]/refresh path
	//     and shows a loading spinner.
	import { onMount } from 'svelte';
	import { SvelteSet } from 'svelte/reactivity';
	import { goto } from '$app/navigation';
	import { useQueryClient } from '@tanstack/svelte-query';
	import {
		useFeeds,
		useEntries,
		useToggleRead,
		useToggleSaved,
		useBulkUpdate,
		keys
	} from '$api/queries';
	import { putJSON, postJSON } from '$api/client';
	import Sidebar from '$lib/components/Sidebar.svelte';
	import TopBar from '$lib/components/TopBar.svelte';
	import RiverList from '$lib/components/RiverList.svelte';
	import MobileTopBar from '$lib/components/MobileTopBar.svelte';
	import MobileTabBar from '$lib/components/MobileTabBar.svelte';
	import OfflineIndicator from '$lib/components/OfflineIndicator.svelte';
	import ConfirmModal from '$lib/components/ConfirmModal.svelte';
	import BulkActionStrip from '$lib/components/BulkActionStrip.svelte';
	import { bindKeyboard } from '$lib/keyboard.svelte';

	const qc = useQueryClient();
	const feeds = useFeeds();
	const entries = useEntries({ status: 'unread' });
	const toggleRead = useToggleRead();
	const toggleSaved = useToggleSaved();
	const bulkUpdate = useBulkUpdate();

	// `userSelectedId` is the explicit user choice (null until they
	// move with j/k). `selectedId` is what the UI actually displays —
	// it falls through to the first visible entry while the user
	// hasn't picked one OR when the previously-picked entry no longer
	// matches anything in the current list (e.g. they just pressed
	// `m` and the entry dropped off the unread filter).
	let userSelectedId = $state<number | null>(null);
	const visible = $derived(entries.data?.data ?? []);
	const total = $derived(entries.data?.pagination.total ?? 0);
	const selectedId = $derived(
		userSelectedId !== null && visible.some((e) => e.id === userSelectedId)
			? userSelectedId
			: (visible[0]?.id ?? null)
	);

	// Multi-select state. The set is reactive via $state — rebuilding
	// the Set on each mutation triggers reactivity for the consumer
	// components. Entering multi-select mode is implicit on the first
	// shift-click / select-box click; explicit Clear empties the set
	// and exits the mode.
	const selectedIds = new SvelteSet<number>();
	const multiSelect = $derived(selectedIds.size > 0);

	// Refresh state. While `refreshing` is true, the button is
	// disabled and shows a spinning glyph (TopBar applies the class).
	let refreshing = $state(false);

	// Mark-all-read confirmation modal.
	let confirmMarkAllOpen = $state(false);

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

	function openMarkAllConfirm() {
		// Don't bring up the modal when there's nothing to mark.
		if (total === 0) return;
		confirmMarkAllOpen = true;
	}

	async function confirmMarkAll() {
		confirmMarkAllOpen = false;
		try {
			await putJSON('/entries/read', {});
			await qc.invalidateQueries({ queryKey: keys.entriesAll() });
		} catch (err) {
			console.error('mark-all-read failed', err);
		}
	}

	async function refresh() {
		// Plan 15 wires this for real. Fan out POST /feeds/[id]/refresh
		// for every subscribed feed; the dispatcher picks them up on
		// its next tick. Invalidate the entries cache after so the
		// river refetches once new rows land.
		const list = feeds.data?.data ?? [];
		if (list.length === 0 || refreshing) return;
		refreshing = true;
		try {
			await Promise.all(list.map((f) => postJSON(`/feeds/${f.id}/refresh`, {})));
			await qc.invalidateQueries({ queryKey: keys.entriesAll() });
		} catch (err) {
			console.error('refresh failed', err);
		} finally {
			refreshing = false;
		}
	}

	function toggleRowSelect(id: number, _ev: MouseEvent) {
		if (selectedIds.has(id)) selectedIds.delete(id);
		else selectedIds.add(id);
	}

	function clearSelection() {
		selectedIds.clear();
	}

	function bulkMark(read: boolean) {
		const ids = Array.from(selectedIds);
		if (ids.length === 0) return;
		bulkUpdate.mutate({ ids, read });
		clearSelection();
	}

	function onRowToggleRead(id: number, read: boolean) {
		toggleRead.mutate({ id, read });
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
				onOpen={(id) => goto('/entry/' + id)}
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
				onMarkAllRead={openMarkAllConfirm}
				onRefresh={refresh}
				{refreshing}
			/>
			{#if multiSelect}
				<BulkActionStrip
					count={selectedIds.size}
					onMarkRead={() => bulkMark(true)}
					onMarkUnread={() => bulkMark(false)}
					onClear={clearSelection}
				/>
			{/if}
			<RiverList
				entries={visible}
				feeds={feeds.data?.data ?? []}
				{selectedId}
				density="default"
				showSummary={true}
				{multiSelect}
				{selectedIds}
				onSelect={(id) => (userSelectedId = id)}
				onOpen={(id) => goto('/entry/' + id)}
				onToggleRead={onRowToggleRead}
				onToggleSelect={toggleRowSelect}
			/>
		</div>
	</div>
{/if}

<OfflineIndicator />

<ConfirmModal
	open={confirmMarkAllOpen}
	title="Mark all read?"
	message={`Mark all ${total} unread ${total === 1 ? 'entry' : 'entries'} as read? This can't be undone.`}
	confirmLabel="Mark all read"
	cancelLabel="Cancel"
	onConfirm={confirmMarkAll}
	onCancel={() => (confirmMarkAllOpen = false)}
/>

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
