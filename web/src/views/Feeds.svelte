<script lang="ts">
  import { onMount } from 'svelte';
  import { subscriptions, categories, entries } from '../lib/store';
  import { filterFeeds, sortFeeds, countByKey, type FilterKey, type SortKey, type FeedRow } from '../lib/feedsFilter';
  import FeedsToolbar from '../components/feeds/FeedsToolbar.svelte';
  import FeedsBulkBar from '../components/feeds/FeedsBulkBar.svelte';
  import FeedsListRow from '../components/feeds/FeedsListRow.svelte';
  import AddFeedDialog from '../components/feeds/AddFeedDialog.svelte';
  import EditFeedDialog from '../components/feeds/EditFeedDialog.svelte';
  import DeleteFeedsDialog from '../components/feeds/DeleteFeedsDialog.svelte';
  import ImportOpmlDialog from '../components/feeds/ImportOpmlDialog.svelte';
  import ExportOpmlDialog from '../components/feeds/ExportOpmlDialog.svelte';
  import CategoryReassignPopover from '../components/CategoryReassignPopover.svelte';
  import EmptyState from '../components/EmptyState.svelte';

  let search = $state('');
  let filter = $state<FilterKey>('all');
  let sort = $state<SortKey>('recent');
  let selected = $state(new Set<number>());
  let expanded = $state(new Set<number>());
  let refreshing = $state(new Set<number>());
  let refreshingAll = $state(false);
  let dialog = $state<
    | null
    | { type: 'add' }
    | { type: 'edit'; feedId: number }
    | { type: 'delete'; feedIds: number[] }
    | { type: 'import' }
    | { type: 'export' }
  >(null);

  let foot = $state<string | null>(null);

  let reassign = $state<{
    open: boolean;
    anchor: HTMLElement | null;
    feedName: string;
    currentCategoryId: number | null;
    label: 'Move' | 'Assign';
    onPick: (id: number | null) => void;
  }>({ open: false, anchor: null, feedName: '', currentCategoryId: null, label: 'Move', onPick: () => {} });

  onMount(() => {
    subscriptions.load();
    categories.load();
    entries.load(true);
  });

  function countUnreadFor(feedId: number): number {
    let n = 0;
    for (const e of $entries.items) if (e.subscription_id === feedId && !e.read) n++;
    return n;
  }

  const decorated: FeedRow[] = $derived.by(() => {
    const now = Math.floor(Date.now() / 1000);
    return $subscriptions.map((s) => ({
      ...s,
      unread: countUnreadFor(s.id),
      lastPollAgo: (s.last_poll_at && s.last_poll_at > 0)
        ? now - s.last_poll_at
        : Number.POSITIVE_INFINITY,
    }));
  });

  const counts = $derived(countByKey(decorated));
  const visible = $derived(sortFeeds(filterFeeds(decorated, filter, search), sort));
  const errorCount = $derived(counts.errors);

  async function refreshOne(id: number) {
    refreshing.add(id);
    refreshing = new Set(refreshing);
    try {
      await subscriptions.refresh(id);
    } catch (e) {
      foot = `Refresh failed: ${(e as Error).message}`;
    } finally {
      refreshing.delete(id);
      refreshing = new Set(refreshing);
    }
  }

  async function refreshAll() {
    refreshingAll = true;
    const ids = visible.map((f) => f.id);
    const results = await subscriptions.refreshMany(ids);
    const failed = results.filter((r) => r.status === 'rejected').length;
    refreshingAll = false;
    if (failed > 0) foot = `Refreshed ${ids.length - failed} of ${ids.length} · ${failed} failed`;
    else if (ids.length > 0) foot = `Refreshed ${ids.length} feed${ids.length === 1 ? '' : 's'}`;
  }

  async function bulkDelete(ids: number[]) {
    const results = await subscriptions.removeMany(ids);
    const failed = results.filter((r) => r.status === 'rejected').length;
    selected = new Set();
    if (failed > 0) foot = `Removed ${ids.length - failed} of ${ids.length} · ${failed} failed`;
    dialog = null;
  }

  async function bulkRefresh(ids: number[]) {
    const results = await subscriptions.refreshMany(ids);
    const failed = results.filter((r) => r.status === 'rejected').length;
    if (failed > 0) foot = `Refreshed ${ids.length - failed} of ${ids.length} · ${failed} failed`;
  }

  async function bulkReassign(ids: number[], categoryId: number | null) {
    const results = await subscriptions.setCategoryMany(ids, categoryId);
    const failed = results.filter((r) => r.status === 'rejected').length;
    selected = new Set();
    if (failed > 0) foot = `Moved ${ids.length - failed} of ${ids.length} · ${failed} failed`;
    dialog = null;
  }

  function openReassignForFeed(feed: FeedRow, anchor: HTMLElement) {
    reassign = {
      open: true, anchor,
      feedName: feed.title,
      currentCategoryId: feed.category_id,
      label: feed.category_id == null ? 'Assign' : 'Move',
      onPick: (id) => { void subscriptions.setCategory(feed.id, id); },
    };
  }

  function openBulkReassign(anchor: HTMLElement) {
    const ids = [...selected];
    reassign = {
      open: true, anchor,
      feedName: `${ids.length} feeds`,
      currentCategoryId: null,
      label: 'Assign',
      onPick: (id) => { void bulkReassign(ids, id); },
    };
  }
</script>

<header class="ts-set-head">
  <div class="ts-set-eyebrow">
    Subscriptions · {decorated.length} feeds · {$categories.length} categories
    {#if errorCount > 0}<span class="err"> · {errorCount} with errors</span>{/if}
  </div>
  <h1 class="ts-set-title">Feeds</h1>
</header>

<div class="ts-feeds">
  <FeedsToolbar
    {search}
    {filter}
    {sort}
    {counts}
    {refreshingAll}
    onSearch={(q) => { search = q; }}
    onFilter={(k) => { filter = k; }}
    onSort={(s) => { sort = s; }}
    onAdd={() => { dialog = { type: 'add' }; }}
    onRefreshAll={refreshAll}
    onImport={() => { dialog = { type: 'import' }; }}
    onExport={() => { dialog = { type: 'export' }; }}
  />

  {#if selected.size > 0}
    <FeedsBulkBar
      count={selected.size}
      onClear={() => { selected = new Set(); }}
      onRefresh={() => bulkRefresh([...selected])}
      onReassign={(anchor) => openBulkReassign(anchor)}
      onDelete={() => { dialog = { type: 'delete', feedIds: [...selected] }; }}
    />
  {/if}

  <div class="ts-feeds-list">
    {#if visible.length === 0 && decorated.length === 0}
      <EmptyState
        title="No feeds yet"
        subtitle="Add a feed by URL, or import an OPML export from another reader."
        cta={{ label: 'Add a feed', onClick: () => { dialog = { type: 'add' }; } }}
      />
    {:else if visible.length === 0}
      <EmptyState
        title="No feeds match"
        subtitle="Nothing matches the current filter."
        cta={{ label: 'Reset filters', onClick: () => { search = ''; filter = 'all'; } }}
      />
    {:else}
      {#each visible as feed (feed.id)}
        <FeedsListRow
          {feed}
          categoryName={feed.category_id ? ($categories.find((c) => c.id === feed.category_id)?.name ?? null) : null}
          isSelected={selected.has(feed.id)}
          anySelected={selected.size > 0}
          isExpanded={expanded.has(feed.id)}
          isRefreshing={refreshing.has(feed.id) || refreshingAll}
          onToggleSelect={() => {
            if (selected.has(feed.id)) selected.delete(feed.id);
            else selected.add(feed.id);
            selected = new Set(selected);
          }}
          onToggleExpand={() => {
            if (expanded.has(feed.id)) expanded.delete(feed.id);
            else expanded.add(feed.id);
            expanded = new Set(expanded);
          }}
          onRefresh={() => refreshOne(feed.id)}
          onChangeCategory={(anchor) => openReassignForFeed(feed, anchor)}
          onEdit={() => { dialog = { type: 'edit', feedId: feed.id }; }}
          onDelete={() => { dialog = { type: 'delete', feedIds: [feed.id] }; }}
        />
      {/each}
    {/if}
  </div>

  <div class="ts-feeds-foot" role="status" aria-live="polite">
    <span class="pulse" aria-hidden="true"></span>
    {#if foot}{foot}{:else}Auto-polling{/if}
    <span class="sep">·</span>
    {decorated.length} feeds across {$categories.length} categories
    {#if errorCount > 0}
      <span class="sep">·</span><span class="err">{errorCount} need attention</span>
    {/if}
  </div>
</div>

{#if dialog?.type === 'add'}
  <AddFeedDialog
    categories={$categories}
    onClose={() => { dialog = null; }}
    onAdded={() => { dialog = null; void subscriptions.load(); }}
  />
{/if}

{#if dialog?.type === 'edit'}
  {@const editDialog = dialog as { type: 'edit'; feedId: number }}
  {@const f = $subscriptions.find((x) => x.id === editDialog.feedId)}
  {#if f}
    <EditFeedDialog
      feed={f}
      categories={$categories}
      onClose={() => { dialog = null; }}
      onSaved={() => { dialog = null; void subscriptions.load(); }}
      onDelete={() => { dialog = { type: 'delete', feedIds: [editDialog.feedId] }; }}
    />
  {/if}
{/if}

{#if dialog?.type === 'delete'}
  {@const deleteDialog = dialog as { type: 'delete'; feedIds: number[] }}
  {@const fs = $subscriptions.filter((x) => deleteDialog.feedIds.includes(x.id))}
  <DeleteFeedsDialog
    feeds={fs}
    onClose={() => { dialog = null; }}
    onConfirm={() => bulkDelete(deleteDialog.feedIds)}
  />
{/if}

{#if dialog?.type === 'import'}
  <ImportOpmlDialog
    onClose={() => { dialog = null; }}
    onImported={() => { void subscriptions.load(); void categories.load(); }}
  />
{/if}

{#if dialog?.type === 'export'}
  <ExportOpmlDialog
    feedCount={decorated.length}
    categoryCount={$categories.length}
    onClose={() => { dialog = null; }}
  />
{/if}

{#if reassign.open}
  <CategoryReassignPopover
    open={reassign.open}
    anchor={reassign.anchor}
    feedName={reassign.feedName}
    currentCategoryId={reassign.currentCategoryId}
    categories={$categories}
    label={reassign.label}
    onPick={(catId) => { reassign.onPick(catId); reassign.open = false; }}
    onClose={() => { reassign.open = false; }}
  />
{/if}

<style>
  .ts-set-head { margin-bottom: 24px; }
  .ts-set-eyebrow { font-family: var(--mono); font-size: 10px; text-transform: uppercase; letter-spacing: 0.1em; color: var(--ink-3); margin-bottom: 4px; }
  .ts-set-title { font-family: var(--serif); font-size: clamp(22px, 3vw, 28px); font-weight: 400; color: var(--ink); margin: 0; }
  .ts-set-head .err { color: var(--color-error-light, #c43a3a); }
  :global(html.theme-dark) .ts-set-head .err { color: var(--color-error-dark, #ec7a7a); }

  .ts-feeds { display: flex; flex-direction: column; }
  .ts-feeds-list { display: flex; flex-direction: column; border: 1px solid var(--rule); border-radius: var(--radius-input, 4px); overflow: hidden; }

  .ts-feeds-foot {
    display: flex; align-items: center; gap: 8px;
    padding: 8px 0; margin-top: 12px;
    font-family: var(--sans); font-size: 12px; color: var(--ink-4);
  }
  .pulse {
    width: 6px; height: 6px; border-radius: 50%;
    background: var(--ink-4);
    flex-shrink: 0;
  }
  .sep { color: var(--ink-4); }
  .err { color: var(--color-error-light, #c43a3a); }
  :global(html.theme-dark) .err { color: var(--color-error-dark, #ec7a7a); }
</style>
