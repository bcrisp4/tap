<script lang="ts">
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { MediaQuery } from 'svelte/reactivity';
	import {
		useEntry,
		useEntries,
		useFeeds,
		useToggleRead,
		useToggleSaved
	} from '$api/queries';
	import Sidebar from '$lib/components/Sidebar.svelte';
	import OfflineIndicator from '$lib/components/OfflineIndicator.svelte';
	import ReaderRail from '$lib/components/reader/ReaderRail.svelte';
	import ReaderHeader from '$lib/components/reader/ReaderHeader.svelte';
	import ReaderBody from '$lib/components/reader/ReaderBody.svelte';
	import MobileReaderTopBar from '$lib/components/reader/MobileReaderTopBar.svelte';
	import MobileReaderFootBar from '$lib/components/reader/MobileReaderFootBar.svelte';

	// TanStack Svelte Query v6 (Plan 09 pin) returns runes-driven reactive
	// objects directly — `entry.isLoading`, `entry.data`. Don't dereference
	// with `$entry.…`; that's the v4 store API and won't work.
	//
	// Pass a getter so the queryKey re-evaluates when the route's `[id]`
	// param changes; capturing the value at call-time would freeze it.
	const entry = useEntry(() => Number(page.params.id));
	const feeds = useFeeds();
	const entries = useEntries({ status: 'unread', limit: 30 });
	const toggleRead = useToggleRead();
	const toggleSaved = useToggleSaved();

	const feed = $derived(
		(feeds.data?.data ?? []).find((f) => f.id === entry.data?.feed_id)
	);
	const railEntries = $derived(entries.data?.data ?? []);

	// Mobile breakpoint follows the design handoff's `.is-mobile` rules
	// (≤ 720 px viewport). Using Svelte's built-in MediaQuery so the
	// reactivity hooks into the rune system without an effect+listener.
	const mobile = new MediaQuery('max-width: 720px');
	const isMobile = $derived(mobile.current);

	let progress = $state(0);

	// Auto-mark-read on reader open. We fire exactly once per entry-id
	// per page lifetime: the user's manual `m` press (or footbar tap on
	// mobile) flips read=false, and the auto-effect must NOT re-fire
	// against that — otherwise the manual unmark looks unresponsive
	// (mobile flicker bug, Plan 18 T3 root cause). Once the auto-mutate
	// has been issued, the user owns the read state.
	const autoMarked = new Set<number>();

	$effect(() => {
		const e = entry.data;
		if (!e) return;
		if (e.read) return;
		if (autoMarked.has(e.id)) return;
		autoMarked.add(e.id);
		toggleRead.mutate({ id: e.id, read: true });
	});

	function back() {
		goto('/');
	}

	function toggleReadHere() {
		const e = entry.data;
		if (!e) return;
		toggleRead.mutate({ id: e.id, read: !e.read });
	}

	function toggleSavedHere() {
		const e = entry.data;
		if (!e) return;
		toggleSaved.mutate({ id: e.id, saved: !e.saved });
	}

	// Reader keyboard shortcuts. `m` is a manual override on top of the
	// auto-mark-read effect: the entry is already read on first press,
	// so `m` flips back to unread.
	function onKey(ev: KeyboardEvent) {
		// Ignore shortcuts while typing in form controls.
		const target = ev.target as HTMLElement | null;
		if (
			target &&
			(target.tagName === 'INPUT' ||
				target.tagName === 'TEXTAREA' ||
				target.isContentEditable)
		) {
			return;
		}
		if (ev.key === 'Escape') {
			// Defer to any open modal (e.g. the image lightbox); Esc
			// should close the dialog before popping the route.
			if (document.querySelector('[role="dialog"][aria-modal="true"]')) {
				return;
			}
			ev.preventDefault();
			back();
			return;
		}
		const e = entry.data;
		if (!e) return;
		if (ev.key === 'm') {
			ev.preventDefault();
			toggleReadHere();
		} else if (ev.key === 's') {
			ev.preventDefault();
			toggleSavedHere();
		} else if (ev.key === 'v' && e.url) {
			ev.preventDefault();
			window.open(e.url, '_blank', 'noopener');
		}
	}

	function onScroll(ev: Event) {
		const el = ev.currentTarget as HTMLElement;
		const max = el.scrollHeight - el.clientHeight;
		progress = max > 0 ? Math.min(1, Math.max(0, el.scrollTop / max)) : 0;
	}
</script>

<svelte:window onkeydown={onKey} />

{#if entry.isLoading}
	<div class="state mono">loading…</div>
{:else if entry.isError || !entry.data}
	<div class="state mono">entry not found</div>
{:else if isMobile}
	<div class="tap is-mobile reader-shell-mobile">
		<MobileReaderTopBar
			progress={progress}
			saved={entry.data.saved}
			onBack={back}
			onSave={toggleSavedHere}
		/>
		<div class="reader-scroller" onscroll={onScroll}>
			<ReaderBody entry={entry.data} feed={feed} />
		</div>
		<MobileReaderFootBar
			read={entry.data.read}
			saved={entry.data.saved}
			entryURL={entry.data.url}
			onToggleRead={toggleReadHere}
			onToggleSaved={toggleSavedHere}
		/>
	</div>
{:else}
	<div class="tap reader-shell">
		<OfflineIndicator />
		<Sidebar />
		<ReaderRail
			entries={railEntries}
			selectedId={entry.data.id}
			collapsed
			onBack={back}
		/>
		<div class="reader-pane">
			<ReaderHeader
				read={entry.data.read}
				saved={entry.data.saved}
				entryURL={entry.data.url}
				onBack={back}
				onToggleRead={toggleReadHere}
				onToggleSaved={toggleSavedHere}
			/>
			<div class="reader-scroller" onscroll={onScroll}>
				<ReaderBody entry={entry.data} feed={feed} />
			</div>
		</div>
	</div>
{/if}

<style>
	.state {
		padding: 80px 24px;
		text-align: center;
		color: var(--ink-3);
		font-size: 12px;
		letter-spacing: 0.04em;
	}
	.reader-shell {
		display: flex;
		height: 100vh;
	}
	.reader-shell-mobile {
		display: flex;
		flex-direction: column;
		height: 100vh;
	}
	.reader-pane {
		flex: 1;
		display: flex;
		flex-direction: column;
		min-width: 0;
		background: var(--bg);
	}
	.reader-scroller {
		flex: 1;
		overflow-y: auto;
	}
</style>
