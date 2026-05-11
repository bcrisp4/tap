<script lang="ts">
  import { onMount, onDestroy, getContext } from 'svelte';
  import EntryRow from '../components/EntryRow.svelte';
  import GroupHeading from '../components/GroupHeading.svelte';
  import Button from '../components/Button.svelte';
  import { entries, subscriptions } from '../lib/store';
  import { navigate } from '../lib/router';
  import { pullToRefresh } from '../lib/pulltorefresh';
  import { bucketByDay } from '../lib/dayBand';
  import { density } from '../lib/preferences.svelte';
  import type { Subscription } from '../lib/types';

  let mainEl = $state<HTMLElement | null>(null);
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
      dispatch.onToggleSaved = () => {
        if (selectedId == null) return;
        const e = $entries.items.find(x => x.id === selectedId);
        if (e) entries.toggleSaved(e.id, !e.saved);
      };
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

  function feedFor(subId: number): Subscription | undefined {
    return $subscriptions.find(s => s.id === subId);
  }

  const bands = $derived(bucketByDay($entries.items));
  const BAND_LABELS = ['Today', 'Yesterday', 'ThisWeek', 'Earlier'] as const;
  const BAND_DISPLAY: Record<typeof BAND_LABELS[number], string> = {
    Today: 'Today', Yesterday: 'Yesterday', ThisWeek: 'This week', Earlier: 'Earlier',
  };
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
  <p class="status">No unread entries. Add a feed from the Feeds tab.</p>
{:else}
  <div
    class="ts-main"
    bind:this={mainEl}
    role="region"
    aria-label="Unread entries"
    {@attach pullToRefresh({
      onRefresh: doRefresh,
      getScrollTop: () => mainEl?.scrollTop ?? 0,
    })}
  >
    {#if refreshing}
      <div class="refresh-indicator" aria-live="polite">
        <span class="pulse" aria-hidden="true"></span>
      </div>
    {/if}
    <ul class="ts-list" role="list" aria-label="Unread entries">
      {#each BAND_LABELS as label (label)}
        {@const bandItems = bands[label]}
        {#if bandItems.length > 0}
          <li class="band-heading" role="presentation">
            <GroupHeading label={BAND_DISPLAY[label]} count={bandItems.length} />
          </li>
          {#each bandItems as entry (entry.id)}
            <li role="listitem">
              <EntryRow
                {entry}
                feed={feedFor(entry.subscription_id)}
                isSelected={selectedId === entry.id}
                density={density.value}
                onToggleRead={() => entries.toggleRead(entry.id, !entry.read)}
                onToggleSaved={() => entries.toggleSaved(entry.id, !entry.saved)}
              />
            </li>
          {/each}
        {/if}
      {/each}
    </ul>
  </div>
{/if}

<style>
  .actions { display: flex; gap: 8px; justify-content: flex-end; margin: 8px 0 16px; }
  .status { padding: 24px; color: var(--ink-3); font-family: var(--mono); font-size: 11px; }
  .status.err { color: #b14; }
  .ts-main { flex: 1; overflow-y: auto; }
  .ts-list { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; }
  .band-heading { list-style: none; }
  .refresh-indicator { display: flex; justify-content: center; padding: 12px 0; }
  .pulse {
    width: 6px; height: 6px; border-radius: 50%; background: var(--accent);
    animation: tap-pulse 2.4s ease-in-out infinite;
  }
</style>
