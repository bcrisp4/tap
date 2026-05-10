<script lang="ts">
  import { onMount, onDestroy, getContext } from 'svelte';
  import Sidebar from '../components/Sidebar.svelte';
  import TopBar from '../components/TopBar.svelte';
  import EntryRow from '../components/EntryRow.svelte';
  import { entries, subscriptions } from '../lib/store';
  import { navigate } from '../lib/router';
  import { pullToRefresh } from '../lib/pulltorefresh';

  let listEl = $state<HTMLElement | null>(null);
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

  function feedFor(subId: number) {
    return $subscriptions.find(s => s.id === subId);
  }
</script>

<div class="layout">
  <Sidebar />
  <main class="main">
    <TopBar
      title="Unread"
      countShown={$entries.items.length}
      countTotal={$entries.items.length}
      onRefresh={doRefresh}
    />
    {#if $entries.loading}
      <p class="status">Loading…</p>
    {:else if $entries.error}
      <p class="status err">{$entries.error}</p>
    {:else if $entries.items.length === 0}
      <p class="status empty">No unread entries. Subscribe to a feed in the sidebar.</p>
    {:else}
      <ul
        class="list"
        role="list"
        aria-label="Unread entries"
        bind:this={listEl}
        {@attach pullToRefresh({
          onRefresh: doRefresh,
          getScrollTop: () => listEl?.scrollTop ?? 0,
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
  </main>
</div>

<style>
  .layout { display: flex; height: 100vh; }
  .main { flex: 1; display: flex; flex-direction: column; overflow-y: auto; background: var(--bg); }
  .status { padding: 24px; color: var(--ink-3); font-family: var(--mono); font-size: 11px; }
  .status.err { color: #b14; }
  .list { flex: 1; list-style: none; margin: 0; padding: 0; }
  .refresh-indicator { display: flex; justify-content: center; padding: 12px 0; }
  .pulse {
    width: 6px; height: 6px; border-radius: 50%; background: var(--accent);
    animation: tap-pulse 2.4s ease-in-out infinite;
  }
</style>
