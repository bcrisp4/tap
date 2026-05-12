<script lang="ts">
  import { onMount } from 'svelte';
  import EntryRow from '../components/EntryRow.svelte';
  import GroupHeading from '../components/GroupHeading.svelte';
  import EmptyState from '../components/EmptyState.svelte';
  import Button from '../components/Button.svelte';
  import { api } from '../lib/api';
  import { subscriptions } from '../lib/store';
  import { bucketByDay } from '../lib/dayBands';
  import type { EntryListItem } from '../lib/types';

  let items = $state<EntryListItem[]>([]);
  let loading = $state(true);
  let error = $state<string | null>(null);
  let loadMoreError = $state<string | null>(null);
  let cursor = $state<string | undefined>(undefined);
  let loadingMore = $state(false);

  const bands = $derived((() => {
    const b = bucketByDay(items);
    return [
      { key: 'today',     label: 'Today',     entries: b.today },
      { key: 'yesterday', label: 'Yesterday', entries: b.yesterday },
      { key: 'thisWeek',  label: 'This week', entries: b.thisWeek },
      { key: 'earlier',   label: 'Earlier',   entries: b.earlier },
    ];
  })());

  function feedFor(subId: number) {
    return $subscriptions.find(s => s.id === subId);
  }

  async function loadInitial() {
    loading = true;
    error = null;
    try {
      const r = await api.listEntries({ limit: 100 });
      items = r.data;
      cursor = r.next_cursor;
    } catch (e) {
      error = (e as Error).message;
    } finally {
      loading = false;
    }
  }

  async function loadMore() {
    if (!cursor || loadingMore) return;
    loadingMore = true;
    loadMoreError = null;
    try {
      const r = await api.listEntries({ limit: 100, cursor });
      items = [...items, ...r.data];
      cursor = r.next_cursor;
    } catch (e) {
      loadMoreError = (e as Error).message;
    } finally {
      loadingMore = false;
    }
  }

  onMount(async () => {
    await subscriptions.load();
    await loadInitial();
  });
</script>

<section class="ts-main" aria-label="History">
  {#if loading}
    <p class="status" role="status">Loading…</p>
  {:else if error}
    <p class="status err" role="alert">{error}</p>
  {:else if items.length === 0}
    <EmptyState
      title="No history yet"
      subtitle="Subscribed feeds will accumulate here as they're polled. Come back after the next poll."
    />
  {:else}
    <div class="ts-list">
      {#each bands as { key, label, entries: bandEntries } (key)}
        {#if bandEntries.length > 0}
          <section data-band={key}>
            <GroupHeading {label} count={bandEntries.length} />
            <ul role="list" aria-label="{label} entries">
              {#each bandEntries as entry (entry.id)}
                <li role="listitem">
                  <EntryRow {entry} feed={feedFor(entry.subscription_id)} />
                </li>
              {/each}
            </ul>
          </section>
        {/if}
      {/each}
    </div>
    {#if cursor}
      <div class="load-more-row">
        <Button
          variant="quiet"
          data-action="load-more"
          disabled={loadingMore}
          onclick={loadMore}
        >
          {loadingMore ? 'Loading…' : 'Load more'}
        </Button>
        {#if loadMoreError}
          <p class="load-more-err" role="alert">{loadMoreError}</p>
        {/if}
      </div>
    {/if}
  {/if}
</section>

<style>
  .ts-main { flex: 1; padding-top: 4px; }
  .status {
    padding: 24px 2px;
    color: var(--ink-3);
    font-family: var(--mono);
    font-size: 11px;
  }
  .status.err { color: #c43a3a; }
  :global(html.theme-dark) .status.err { color: #ec7a7a; }
  .ts-list {
    display: flex;
    flex-direction: column;
  }
  .ts-list ul {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
  }
  .ts-list li { display: contents; }
  .load-more-row {
    display: flex;
    flex-direction: column;
    align-items: center;
    padding: 24px 0;
    gap: 8px;
  }
  .load-more-err {
    font-family: var(--mono);
    font-size: 11px;
    color: #c43a3a;
  }
  :global(html.theme-dark) .load-more-err { color: #ec7a7a; }
</style>
