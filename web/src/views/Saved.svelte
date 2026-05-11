<script lang="ts">
  import { onMount } from 'svelte';
  import Sidebar from '../components/Sidebar.svelte';
  import TopBar from '../components/TopBar.svelte';
  import EntryRow from '../components/EntryRow.svelte';
  import { api } from '../lib/api';
  import type { EntryListItem } from '../lib/types';

  let items = $state<EntryListItem[]>([]);
  let loading = $state(true);
  let error = $state<string | null>(null);

  async function load() {
    loading = true; error = null;
    try {
      const r = await api.listEntries({ saved: true, limit: 100 });
      items = r.data;
    } catch (e) {
      error = (e as Error).message;
    } finally {
      loading = false;
    }
  }

  onMount(load);
</script>

<div class="layout">
  <Sidebar />
  <main class="saved-main">
    <TopBar
      title="Saved"
      countShown={items.length}
      countTotal={items.length}
      onRefresh={load}
    />
    {#if loading}
      <p class="status">Loading…</p>
    {:else if error}
      <p class="status err">{error}</p>
    {:else if items.length === 0}
      <p class="status">No saved entries yet.</p>
    {:else}
      <ul class="list" role="list" aria-label="Saved entries">
        {#each items as e (e.id)}
          <li role="listitem">
            <EntryRow entry={e} feed={undefined} />
          </li>
        {/each}
      </ul>
    {/if}
  </main>
</div>

<style>
  .layout { display: flex; height: 100vh; }
  .saved-main { flex: 1; display: flex; flex-direction: column; overflow-y: auto; background: var(--bg); }
  .status { padding: 24px; color: var(--ink-3); font-family: var(--mono); font-size: 11px; }
  .status.err { color: #b14; }
  .list { flex: 1; list-style: none; margin: 0; padding: 0; }
</style>
