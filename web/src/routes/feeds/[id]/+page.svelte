<script lang="ts">
	// /feeds/[id] — feed detail + edit + refresh + delete.
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import {
		useFeed,
		useEntries,
		useDeleteFeed,
		useUpdateFeed,
		useRefreshFeed
	} from '$api/queries';
	import { ApiError } from '$api/client';
	import type { FeedPatch } from '$api/types';
	import { isMobile } from '$lib/breakpoints.svelte';
	import Sidebar from '$lib/components/Sidebar.svelte';
	import TopBar from '$lib/components/TopBar.svelte';
	import RiverList from '$lib/components/RiverList.svelte';
	import FeedForm from '$lib/components/FeedForm.svelte';
	import MobileTopBar from '$lib/components/MobileTopBar.svelte';
	import MobileTabBar from '$lib/components/MobileTabBar.svelte';

	// SvelteKit remounts the route component on dynamic-param changes,
	// so capturing `page.params.id` once at mount time is safe — a
	// navigation to a sibling /feeds/N gives a fresh evaluation. We
	// still read `page.params.id` lazily inside the useFeed getter so
	// the queryKey stays in sync with the URL within the same mount.
	const feedId = Number(page.params.id);
	const feed = useFeed(() => Number(page.params.id));
	const entries = useEntries({ status: 'all', feed_id: feedId, limit: 50 });

	const updateMut = useUpdateFeed();
	const deleteMut = useDeleteFeed();
	const refreshMut = useRefreshFeed();

	let editing = $state(false);
	let editError = $state<string | null>(null);
	let deleteOpen = $state(false);
	let deleteConfirmText = $state('');
	let refreshFlash = $state(false);

	const visible = $derived(entries.data?.data ?? []);
	const total = $derived(entries.data?.pagination.total ?? 0);

	function formatTime(unix: number | null | undefined): string {
		if (!unix || unix <= 0) return 'never';
		try {
			return new Date(unix * 1000).toLocaleString(undefined, {
				dateStyle: 'medium',
				timeStyle: 'short'
			});
		} catch {
			return '—';
		}
	}

	function refresh() {
		refreshMut.mutate(feedId, {
			onSuccess: () => {
				refreshFlash = true;
				window.setTimeout(() => {
					refreshFlash = false;
				}, 1500);
			}
		});
	}

	function handleUpdate(patch: FeedPatch) {
		editError = null;
		updateMut.mutate(
			{ id: feedId, patch },
			{
				onSuccess: () => {
					editing = false;
				},
				onError: (err) => {
					editError = err instanceof ApiError ? err.message : 'save failed';
				}
			}
		);
	}

	function confirmDelete() {
		// Type-the-title-to-confirm pattern.
		if (!feed.data) return;
		if (deleteConfirmText.trim() !== feed.data.title) return;
		deleteMut.mutate(feedId, {
			onSuccess: () => goto('/'),
			onError: (err) => {
				editError = err instanceof ApiError ? err.message : 'delete failed';
			}
		});
	}

	const mobile = $derived(isMobile());
</script>

<svelte:head>
	<title>{feed.data?.title ?? 'Feed'} — tap</title>
</svelte:head>

{#snippet content()}
	<div class="feed-wrap">
		{#if feed.isLoading}
			<p class="empty mono">loading…</p>
		{:else if feed.isError}
			<p class="empty mono">feed not found</p>
		{:else if feed.data}
			<header class="feed-head">
				<div class="head-top">
					<h1 class="title">{feed.data.title}</h1>
					{#if feed.data.disabled}
						<span class="tag mono">paused</span>
					{/if}
				</div>
				<a class="url mono" href={feed.data.feed_url} target="_blank" rel="noopener noreferrer">
					{feed.data.feed_url}
				</a>
				<dl class="metrics">
					<div>
						<dt>Last polled</dt>
						<dd class="mono">{formatTime(feed.data.last_polled_at)}</dd>
					</div>
					<div>
						<dt>This week</dt>
						<dd class="mono">{feed.data.weekly_entry_count} entries</dd>
					</div>
					<div>
						<dt>Errors</dt>
						<dd class="mono">{feed.data.error_count}</dd>
					</div>
				</dl>
				{#if feed.data.last_error}
					<p class="last-err mono">{feed.data.last_error}</p>
				{/if}

				<div class="actions">
					<button
						type="button"
						class="btn ghost"
						onclick={refresh}
						disabled={refreshMut.isPending}
						data-testid="feed-refresh-btn"
					>
						{refreshMut.isPending ? 'refreshing…' : refreshFlash ? 'refresh queued ✓' : 'Refresh'}
					</button>
					<button
						type="button"
						class="btn ghost"
						onclick={() => (editing = !editing)}
						data-testid="feed-edit-btn"
					>
						{editing ? 'Close edit' : 'Edit'}
					</button>
					<button
						type="button"
						class="btn danger"
						onclick={() => (deleteOpen = true)}
						data-testid="feed-delete-btn"
					>
						Delete
					</button>
				</div>
			</header>

			{#if editing}
				<section class="edit-card">
					<h2>Edit feed</h2>
					<FeedForm
						mode="edit"
						feed={feed.data}
						busy={updateMut.isPending}
						errorMessage={editError}
						onUpdate={handleUpdate}
						onCancel={() => (editing = false)}
					/>
				</section>
			{/if}

			<section class="entries">
				<h2>Recent entries · <span class="mono">{total}</span></h2>
				{#if visible.length === 0}
					<p class="empty mono">no entries yet · the poller may still be catching up</p>
				{:else}
					<RiverList
						entries={visible}
						feeds={feed.data ? [feed.data] : []}
						density="compact"
						showSummary={false}
						onSelect={(id) => goto('/entry/' + id)}
					/>
				{/if}
			</section>
		{/if}
	</div>

	{#if deleteOpen && feed.data}
		<div class="modal-backdrop">
			<button
				type="button"
				class="modal-scrim"
				aria-label="Close confirmation"
				onclick={() => {
					deleteOpen = false;
					deleteConfirmText = '';
				}}
			></button>
			<div class="modal" role="dialog" aria-modal="true" aria-labelledby="del-title">
				<h3 id="del-title">Delete this feed?</h3>
				<p>
					This unsubscribes from <b>{feed.data.title}</b> and removes its entries from the
					river. Type the title below to confirm.
				</p>
				<input
					type="text"
					bind:value={deleteConfirmText}
					placeholder={feed.data.title}
					data-testid="feed-delete-confirm"
				/>
				<div class="modal-actions">
					<button
						type="button"
						class="btn ghost"
						onclick={() => {
							deleteOpen = false;
							deleteConfirmText = '';
						}}
					>
						Cancel
					</button>
					<button
						type="button"
						class="btn danger"
						disabled={deleteConfirmText.trim() !== feed.data.title || deleteMut.isPending}
						onclick={confirmDelete}
						data-testid="feed-delete-confirm-btn"
					>
						{deleteMut.isPending ? 'deleting…' : 'Delete feed'}
					</button>
				</div>
			</div>
		</div>
	{/if}
{/snippet}

{#if mobile}
	<div class="tap is-mobile">
		<MobileTopBar label={feed.data?.title ?? 'feed'} />
		<div class="m-wrap">{@render content()}</div>
		<MobileTabBar />
	</div>
{:else}
	<div class="tap">
		<Sidebar />
		<div class="col">
			<TopBar title={feed.data?.title ?? 'Feed'} />
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

	.feed-wrap {
		max-width: 880px;
		margin: 0 auto;
		padding: 32px 28px 80px;
		display: flex;
		flex-direction: column;
		gap: 28px;
	}
	.feed-head {
		display: flex;
		flex-direction: column;
		gap: 14px;
		padding-bottom: 24px;
		border-bottom: 1px solid var(--rule);
	}
	.head-top {
		display: flex;
		align-items: baseline;
		gap: 12px;
		flex-wrap: wrap;
	}
	.title {
		font-family: var(--serif);
		font-size: 30px;
		font-weight: 500;
		letter-spacing: -0.015em;
		margin: 0;
		color: var(--ink);
	}
	.tag {
		font-size: 10px;
		letter-spacing: 0.08em;
		text-transform: uppercase;
		color: var(--ink-3);
		padding: 2px 8px;
		border: 1px solid var(--ink-4);
		border-radius: 3px;
	}
	.url {
		font-size: 12px;
		color: var(--ink-3);
		text-decoration: none;
		word-break: break-all;
	}
	.url:hover {
		color: var(--accent);
	}
	.metrics {
		display: flex;
		flex-wrap: wrap;
		gap: 28px;
		margin: 4px 0 0;
	}
	.metrics div {
		display: flex;
		flex-direction: column;
		gap: 2px;
	}
	.metrics dt {
		font-family: var(--sans);
		font-size: 10.5px;
		letter-spacing: 0.08em;
		text-transform: uppercase;
		color: var(--ink-3);
	}
	.metrics dd {
		margin: 0;
		font-size: 14px;
		color: var(--ink);
	}
	.last-err {
		margin: 0;
		padding: 8px 12px;
		font-size: 11.5px;
		color: #c1282d;
		background: rgba(193, 40, 45, 0.08);
		border-left: 2px solid #c1282d;
		border-radius: 0 4px 4px 0;
	}
	.actions {
		display: flex;
		gap: 8px;
		flex-wrap: wrap;
		padding-top: 6px;
	}
	.btn {
		font-family: var(--sans);
		font-size: 13px;
		padding: 8px 14px;
		border-radius: 5px;
		cursor: pointer;
		border: 1px solid var(--rule);
		background: transparent;
		color: var(--ink-2);
	}
	.btn:hover:not(:disabled) {
		color: var(--ink);
		border-color: var(--ink-4);
	}
	.btn:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}
	.btn.danger {
		color: #c1282d;
		border-color: rgba(193, 40, 45, 0.4);
	}
	.btn.danger:hover:not(:disabled) {
		background: rgba(193, 40, 45, 0.08);
		border-color: #c1282d;
	}

	.edit-card {
		background: var(--surface);
		border: 1px solid var(--rule);
		border-radius: 8px;
		padding: 22px 24px;
		display: flex;
		flex-direction: column;
		gap: 18px;
	}
	.edit-card h2 {
		font-family: var(--serif);
		font-size: 20px;
		font-weight: 500;
		margin: 0;
	}

	.entries h2 {
		font-family: var(--sans);
		font-size: 13px;
		font-weight: 600;
		text-transform: uppercase;
		letter-spacing: 0.04em;
		color: var(--ink-2);
		margin: 0 0 12px;
	}

	.empty {
		padding: 60px 24px;
		text-align: center;
		color: var(--ink-3);
		font-size: 11px;
		letter-spacing: 0.06em;
		text-transform: uppercase;
	}

	.modal-backdrop {
		position: fixed;
		inset: 0;
		display: grid;
		place-items: center;
		z-index: 100;
		padding: 20px;
	}
	.modal-scrim {
		position: absolute;
		inset: 0;
		background: rgba(0, 0, 0, 0.4);
		border: 0;
		padding: 0;
		cursor: pointer;
	}
	.modal {
		position: relative;
		background: var(--surface);
		border-radius: 8px;
		padding: 28px 28px 22px;
		width: min(440px, 100%);
		display: flex;
		flex-direction: column;
		gap: 14px;
		box-shadow: 0 20px 60px rgba(0, 0, 0, 0.25);
	}
	.modal h3 {
		font-family: var(--serif);
		font-size: 22px;
		font-weight: 500;
		margin: 0;
		color: var(--ink);
	}
	.modal p {
		font-family: var(--serif);
		font-size: 14px;
		line-height: 1.5;
		color: var(--ink-2);
		margin: 0;
	}
	.modal input {
		font-family: var(--serif);
		font-size: 15px;
		padding: 10px 12px;
		background: var(--bg);
		border: 1px solid var(--rule);
		border-radius: 5px;
		color: var(--ink);
	}
	.modal input:focus {
		outline: 0;
		border-color: var(--accent);
		box-shadow: 0 0 0 3px var(--accent-soft);
	}
	.modal-actions {
		display: flex;
		gap: 10px;
		justify-content: flex-end;
	}
</style>
