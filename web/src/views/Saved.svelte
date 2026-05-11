<script lang="ts">
  import { onMount, onDestroy, getContext } from 'svelte';
  import SavedToolbar from '../components/SavedToolbar.svelte';
  import SavedRow from '../components/SavedRow.svelte';
  import SavedMobileRow from '../components/SavedMobileRow.svelte';
  import EmptyState from '../components/EmptyState.svelte';
  import KbdChip from '../components/KbdChip.svelte';
  import { entries, subscriptions } from '../lib/store';
  import { navigate } from '../lib/router';
  import { isMobile } from '../lib/breakpoints.svelte';
  import type { Subscription } from '../lib/types';

  let focusedId = $state<number | null>(null);

  const items = $derived($entries.items.filter(e => e.saved));

  const dispatch = getContext<{
    onNext: () => void; onPrev: () => void; onOpen: () => void;
    onToggleRead: () => void; onToggleSaved: () => void;
  } | undefined>('keyDispatch');

  function feedFor(subId: number): Subscription | undefined {
    return $subscriptions.find(s => s.id === subId);
  }

  onMount(() => {
    entries.loadSaved();
    subscriptions.load();

    if (dispatch) {
      dispatch.onToggleSaved = () => {
        if (focusedId != null) entries.toggleSaved(focusedId, false);
      };
      dispatch.onToggleRead = () => {
        if (focusedId == null) return;
        const e = items.find(x => x.id === focusedId);
        if (e) entries.toggleRead(e.id, !e.read);
      };
      dispatch.onOpen = () => {
        if (focusedId != null) navigate(`/entry/${focusedId}`);
      };
    }
  });

  onDestroy(() => {
    if (dispatch) {
      dispatch.onToggleSaved = () => {};
      dispatch.onToggleRead = () => {};
      dispatch.onOpen = () => {};
    }
  });
</script>

{#snippet emptySubtitle()}
  Press <KbdChip>S</KbdChip> on any entry to keep it here.
{/snippet}

{#if $entries.loading}
  <p class="status" role="status">Loading…</p>
{:else if $entries.error}
  <p class="status err" role="alert">{$entries.error}</p>
{:else if items.length === 0}
  <EmptyState
    dot="accent"
    title="Nothing saved yet"
    subtitle={emptySubtitle}
  />
{:else}
  <SavedToolbar count={items.length} />
  <ul class="list" role="list" aria-label="Saved entries">
    {#each items as entry (entry.id)}
      {@const RowComponent = $isMobile ? SavedMobileRow : SavedRow}
      <li>
        <RowComponent
          {entry}
          feed={feedFor(entry.subscription_id)}
          isFocused={focusedId === entry.id}
          onFocus={() => (focusedId = entry.id)}
          onMouseEnter={() => (focusedId = entry.id)}
          onOpen={() => navigate(`/entry/${entry.id}`)}
          onUnsave={() => entries.toggleSaved(entry.id, false)}
          onToggleRead={() => entries.toggleRead(entry.id, !entry.read)}
        />
      </li>
    {/each}
  </ul>
{/if}

<style>
  .status {
    padding: 24px 2px;
    color: var(--ink-3);
    font-family: var(--mono);
    font-size: 11px;
  }
  .status.err { color: #c43a3a; }

  .list {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
  }
</style>
