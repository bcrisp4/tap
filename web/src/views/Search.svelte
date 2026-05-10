<script lang="ts">
  import { onMount } from 'svelte';
  import Sidebar from '../components/Sidebar.svelte';
  import EntryRow from '../components/EntryRow.svelte';
  import { api } from '../lib/api';
  import type { EntryListItem } from '../lib/types';

  let query = $state('');
  let results = $state<EntryListItem[]>([]);
  let loading = $state(false);
  let error = $state('');
  let inputEl = $state<HTMLInputElement | null>(null);

  let debounceTimer: ReturnType<typeof setTimeout> | null = null;
  let currentController: AbortController | null = null;

  function syncURL(q: string) {
    const url = new URL(window.location.href);
    if (q) {
      url.searchParams.set('q', q);
    } else {
      url.searchParams.delete('q');
    }
    window.history.replaceState({}, '', url.toString());
  }

  async function doSearch(q: string) {
    if (q.length < 3) {
      results = [];
      return;
    }
    // Cancel any previous in-flight request.
    currentController?.abort();
    currentController = new AbortController();
    const { signal } = currentController;

    loading = true;
    error = '';
    try {
      const resp = await api.searchEntries(q);
      if (!signal.aborted) {
        results = resp.data;
      }
    } catch (e) {
      if (!signal.aborted) {
        error = e instanceof Error ? e.message : 'Search failed';
        results = [];
      }
    } finally {
      if (!signal.aborted) {
        loading = false;
      }
    }
  }

  function onInput() {
    const q = query;
    syncURL(q);
    if (debounceTimer !== null) clearTimeout(debounceTimer);
    debounceTimer = setTimeout(() => doSearch(q), 300);
  }

  onMount(() => {
    const params = new URLSearchParams(window.location.search);
    const q = params.get('q') ?? '';
    if (q) {
      query = q;
      void doSearch(q);
    }
    inputEl?.focus();

    const onFocusSearch = () => inputEl?.focus();
    window.addEventListener('tap:focus-search', onFocusSearch);

    return () => {
      if (debounceTimer !== null) clearTimeout(debounceTimer);
      window.removeEventListener('tap:focus-search', onFocusSearch);
    };
  });
</script>

<div class="layout">
  <Sidebar />
  <main class="main">
    <div class="search-bar">
      <input
        bind:this={inputEl}
        bind:value={query}
        oninput={onInput}
        class="search-input"
        type="search"
        placeholder="Search entries..."
        aria-label="Search entries"
      />
    </div>
    {#if query.length > 0 && query.length < 3}
      <p class="hint">Type at least 3 characters to search.</p>
    {:else if loading}
      <p class="hint">Searching...</p>
    {:else if error}
      <p class="hint error">{error}</p>
    {:else if results.length === 0 && query.length >= 3}
      <p class="hint">No results for "{query}".</p>
    {:else}
      <ul class="entry-list" aria-label="Search results">
        {#each results as entry (entry.id)}
          <EntryRow {entry} feed={undefined} />
        {/each}
      </ul>
    {/if}
  </main>
</div>

<style>
  .layout { display: flex; height: 100vh; overflow: hidden; }
  .main { flex: 1; display: flex; flex-direction: column; background: var(--bg); overflow-y: auto; }
  .search-bar { padding: 16px 20px; border-bottom: 1px solid var(--rule); }
  .search-input {
    width: 100%;
    padding: 8px 12px;
    font-family: var(--sans);
    font-size: 14px;
    background: var(--bg-2, var(--bg));
    color: var(--ink);
    border: 1px solid var(--rule);
    border-radius: 4px;
    outline: none;
    box-sizing: border-box;
  }
  .search-input:focus { border-color: var(--accent); }
  .hint { color: var(--ink-3); font-family: var(--mono); font-size: 11px; padding: 20px; text-align: center; }
  .hint.error { color: var(--error, #c0392b); }
  .entry-list { list-style: none; margin: 0; padding: 0; }
</style>
