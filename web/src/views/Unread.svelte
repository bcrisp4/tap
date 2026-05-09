<script lang="ts">
  import { onMount } from 'svelte';
  import Sidebar from '../components/Sidebar.svelte';
  import TopBar from '../components/TopBar.svelte';
  import EntryRow from '../components/EntryRow.svelte';
  import { entries, subscriptions } from '../lib/store';

  onMount(() => {
    entries.load(true);
    subscriptions.load();
  });

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
      onRefresh={() => entries.load(true)}
    />
    {#if $entries.loading}
      <p class="status">Loading…</p>
    {:else if $entries.error}
      <p class="status err">{$entries.error}</p>
    {:else if $entries.items.length === 0}
      <p class="status empty">No unread entries. Subscribe to a feed in the sidebar.</p>
    {:else}
      <div class="list">
        {#each $entries.items as entry (entry.id)}
          <EntryRow {entry} feed={feedFor(entry.subscription_id)} />
        {/each}
      </div>
    {/if}
  </main>
</div>

<style>
  .layout {
    display: flex;
    height: 100vh;
  }
  .main {
    flex: 1;
    display: flex;
    flex-direction: column;
    overflow-y: auto;
    background: var(--bg);
  }
  .status {
    padding: 24px;
    color: var(--ink-3);
    font-family: var(--mono);
    font-size: 11px;
  }
  .status.err { color: #b14; }
  .list { flex: 1; }
</style>
