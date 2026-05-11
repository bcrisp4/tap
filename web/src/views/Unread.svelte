<script lang="ts">
  import { onMount, onDestroy, getContext } from 'svelte';
  import EntryRow from '../components/EntryRow.svelte';
  import Button from '../components/Button.svelte';
  import EmptyState from '../components/EmptyState.svelte';
  import { entries, subscriptions } from '../lib/store';
  import { navigate } from '../lib/router';
  import { pullToRefresh } from '../lib/pulltorefresh';

  let refreshing = $state(false);
  let selectedId = $state<number | null>(null);

  const dispatch = getContext<{
    onNext: () => void; onPrev: () => void; onOpen: () => void;
    onToggleRead: () => void; onToggleSaved: () => void;
  }>('keyDispatch');

  onMount(() => {
    entries.load(true);
    subscriptions.load();

    if (dispatch) {
      dispatch.onNext = () => {
        const items = $entries.items;
        if (!items.length) return;
        const idx = selectedId == null ? -1 : items.findIndex(e => e.id === selectedId);
        selectedId = items[Math.min(idx + 1, items.length - 1)].id;
      };
      dispatch.onPrev = () => {
        const items = $entries.items;
        if (!items.length) return;
        const idx = selectedId == null ? items.length : items.findIndex(e => e.id === selectedId);
        selectedId = items[Math.max(idx - 1, 0)].id;
      };
      dispatch.onOpen = () => { if (selectedId != null) navigate(`/entry/${selectedId}`); };
      dispatch.onToggleRead = () => {
        if (selectedId == null) return;
        const e = $entries.items.find(x => x.id === selectedId);
        if (e) entries.toggleRead(e.id, !e.read);
      };
      dispatch.onToggleSaved = () => {};
    }
  });

  onDestroy(() => {
    if (dispatch) {
      dispatch.onNext = () => {};
      dispatch.onPrev = () => {};
      dispatch.onOpen = () => {};
      dispatch.onToggleRead = () => {};
      dispatch.onToggleSaved = () => {};
    }
  });

  async function doRefresh() {
    refreshing = true;
    try { await entries.load(true); } finally { refreshing = false; }
  }

  async function markAllRead() {
    const ids = $entries.items.map(e => e.id);
    await Promise.allSettled(ids.map(id => entries.toggleRead(id, true)));
  }

  function feedFor(subId: number) {
    return $subscriptions.find(s => s.id === subId);
  }
</script>

<div class="actions">
  <Button variant="quiet" onclick={doRefresh}>Refresh</Button>
  <Button variant="quiet" onclick={markAllRead}>Mark all read</Button>
</div>

{#if $entries.loading}
  <p class="status">Loading…</p>
{:else if $entries.error}
  <p class="status err">{$entries.error}</p>
{:else if $entries.items.length === 0}
  <EmptyState title="No unread entries." subtitle="Add a feed from the Feeds tab." />
{:else}
  <ul
    class="list"
    role="list"
    aria-label="Unread entries"
    {@attach pullToRefresh({
      onRefresh: doRefresh,
      getScrollTop: () => window.scrollY,
    })}
  >
    {#if refreshing}
      <li class="refresh-indicator" aria-live="polite">
        <span class="pulse" aria-hidden="true"></span>
      </li>
    {/if}
    {#each $entries.items as entry (entry.id)}
      <li role="listitem">
        <EntryRow
          {entry}
          feed={feedFor(entry.subscription_id)}
          isSelected={selectedId === entry.id}
          onToggleRead={() => entries.toggleRead(entry.id, !entry.read)}
          onToggleSaved={() => {}}
        />
      </li>
    {/each}
  </ul>
{/if}

<style>
  .actions { display: flex; gap: 8px; justify-content: flex-end; margin: 8px 0 16px; }
  .status { padding: 24px; color: var(--ink-3); font-family: var(--mono); font-size: 11px; }
  .status.err { color: #b14; }
  .list { flex: 1; list-style: none; margin: 0; padding: 0; }
  .refresh-indicator { display: flex; justify-content: center; padding: 12px 0; }
  .pulse {
    width: 6px; height: 6px; border-radius: 50%; background: var(--accent);
    animation: tap-pulse 2.4s ease-in-out infinite;
  }
</style>
