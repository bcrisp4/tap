<script lang="ts">
  import Sidebar from '../components/Sidebar.svelte';
  import EntryRow from '../components/EntryRow.svelte';
  import { api } from '../lib/api';
  import type { EntryListItem, Category } from '../lib/types';

  let { id }: { id: number } = $props();

  let category = $state<Category | null>(null);
  let entries = $state<EntryListItem[]>([]);
  let loading = $state(true);
  let error = $state('');
  let markingRead = $state(false);
  let confirmOpen = $state(false);

  async function load() {
    loading = true;
    error = '';
    try {
      const [cats, resp] = await Promise.all([
        api.listCategories(),
        api.listEntries({ category: id, unread: true }),
      ]);
      category = cats.find(c => c.id === id) ?? null;
      entries = resp.data;
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to load';
    } finally {
      loading = false;
    }
  }

  async function markAllRead() {
    markingRead = true;
    try {
      await api.markCategoryRead(id);
      entries = entries.map(e => ({ ...e, read: true }));
      confirmOpen = false;
      // Reload to get fresh unread count.
      await load();
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to mark read';
    } finally {
      markingRead = false;
    }
  }

  // Re-load when id prop changes so navigating category-to-category works correctly.
  $effect(() => {
    category = null;
    entries = [];
    confirmOpen = false;
    error = '';
    void load();
  });
</script>

<div class="layout">
  <Sidebar />
  <main class="main">
    <div class="topbar">
      <div class="title-row">
        <h1 class="title">{category?.name ?? '...'}</h1>
        {#if category && category.unread > 0}
          <span class="unread-count">{category.unread}</span>
        {/if}
      </div>
      {#if category && category.unread > 0}
        {#if confirmOpen}
          <div class="confirm-row">
            <span class="confirm-label">Mark all {category.unread} as read?</span>
            <button class="btn-confirm" onclick={markAllRead} disabled={markingRead}>Yes, mark all read</button>
            <button class="btn-cancel" onclick={() => { confirmOpen = false; }}>Cancel</button>
          </div>
        {:else}
          <button class="btn-mark-read" onclick={() => { confirmOpen = true; }}>Mark all read</button>
        {/if}
      {/if}
    </div>
    {#if loading}
      <p class="hint">Loading...</p>
    {:else if error}
      <p class="hint error">{error}</p>
    {:else if entries.length === 0}
      <p class="hint">No unread entries in this category.</p>
    {:else}
      <ul class="entry-list" aria-label="Category entries">
        {#each entries as entry (entry.id)}
          <EntryRow {entry} feed={undefined} />
        {/each}
      </ul>
    {/if}
  </main>
</div>

<style>
  .layout { display: flex; height: 100vh; overflow: hidden; }
  .main { flex: 1; display: flex; flex-direction: column; background: var(--bg); overflow-y: auto; }
  .topbar {
    padding: 12px 20px;
    border-bottom: 1px solid var(--rule);
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
  .title-row { display: flex; align-items: center; gap: 8px; }
  .title { margin: 0; font-family: var(--sans); font-size: 16px; font-weight: 600; color: var(--ink); }
  .unread-count {
    font-family: var(--mono);
    font-size: 11px;
    color: var(--accent);
    background: color-mix(in srgb, var(--accent) 15%, transparent);
    padding: 2px 6px;
    border-radius: 10px;
  }
  .confirm-row { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
  .confirm-label { font-family: var(--sans); font-size: 12px; color: var(--ink-2); }
  .btn-mark-read, .btn-confirm, .btn-cancel {
    font-family: var(--sans);
    font-size: 12px;
    padding: 4px 10px;
    border-radius: 4px;
    cursor: pointer;
    border: 1px solid var(--rule);
    background: var(--bg);
    color: var(--ink);
  }
  .btn-confirm { background: var(--accent); color: #fff; border-color: var(--accent); }
  .btn-confirm:disabled { opacity: 0.6; cursor: not-allowed; }
  .hint { color: var(--ink-3); font-family: var(--mono); font-size: 11px; padding: 20px; text-align: center; }
  .hint.error { color: var(--error, #c0392b); }
  .entry-list { list-style: none; margin: 0; padding: 0; }
</style>
