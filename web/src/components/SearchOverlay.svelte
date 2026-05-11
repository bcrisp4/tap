<script lang="ts">
  import { searchOverlay } from '../lib/searchOverlay.svelte';
  import { api } from '../lib/api';
  import { navigate } from '../lib/router';
  import type { EntryListItem } from '../lib/types';

  let inputEl = $state<HTMLInputElement | null>(null);
  let results = $state<EntryListItem[]>([]);
  let loading = $state(false);
  let error = $state<string | null>(null);
  let debounceTimer: ReturnType<typeof setTimeout> | null = null;
  let restoreFocusTo: HTMLElement | null = null;

  $effect(() => {
    if (searchOverlay.open) {
      restoreFocusTo = document.activeElement as HTMLElement | null;
      queueMicrotask(() => inputEl?.focus());
    } else {
      results = []; error = null; loading = false;
      if (debounceTimer !== null) { clearTimeout(debounceTimer); debounceTimer = null; }
      restoreFocusTo?.focus?.();
      restoreFocusTo = null;
    }
  });

  async function runSearch(q: string) {
    if (q.length < 3) { results = []; return; }
    loading = true; error = null;
    try {
      const resp = await api.searchEntries(q);
      results = resp.data;
    } catch (e) {
      error = (e as Error).message; results = [];
    } finally {
      loading = false;
    }
  }

  function onInput(ev: Event) {
    const q = (ev.target as HTMLInputElement).value;
    searchOverlay.setQuery(q);
    if (debounceTimer !== null) clearTimeout(debounceTimer);
    debounceTimer = setTimeout(() => runSearch(q), 250);
  }

  function pickResult(id: number) {
    navigate(`/entry/${id}`);
    searchOverlay.close();
  }

  function onKeyDown(ev: KeyboardEvent) {
    if (ev.key === 'Escape') { searchOverlay.close(); }
  }

  function onScrimClick(ev: MouseEvent) {
    if (ev.target === ev.currentTarget) searchOverlay.close();
  }
</script>

{#if searchOverlay.open}
  <!-- svelte-ignore a11y_no_static_element_interactions a11y_click_events_have_key_events -->
  <div class="search-overlay" role="dialog" aria-modal="true" aria-label="Search entries"
       tabindex="-1" onclick={onScrimClick} onkeydown={onKeyDown}>
    <div class="panel" onclick={(e) => e.stopPropagation()}>
      <input
        bind:this={inputEl}
        type="search"
        placeholder="Search…"
        value={searchOverlay.query}
        oninput={onInput}
        aria-label="Search entries"
        class="input"
      />
      {#if searchOverlay.query.length > 0 && searchOverlay.query.length < 3}
        <p class="hint">Type at least 3 characters.</p>
      {:else if loading}
        <p class="hint">Searching…</p>
      {:else if error}
        <p class="hint err">{error}</p>
      {:else if results.length === 0 && searchOverlay.query.length >= 3}
        <p class="hint">No results.</p>
      {:else}
        <ul class="results" role="list">
          {#each results as r (r.id)}
            <li role="listitem">
              <button type="button" class="result" onclick={() => pickResult(r.id)}>{r.title}</button>
            </li>
          {/each}
        </ul>
      {/if}
    </div>
  </div>
{/if}

<style>
  .search-overlay {
    position: fixed; inset: 0;
    background: rgba(0, 0, 0, 0.32);
    display: flex; align-items: flex-start; justify-content: center;
    padding-top: 80px;
    z-index: 300;
  }
  .panel {
    width: min(560px, 92vw);
    background: var(--bg);
    border: 1px solid var(--rule);
    border-radius: 6px;
    box-shadow: 0 10px 30px rgba(0,0,0,0.18);
    padding: 14px;
  }
  .input {
    width: 100%; box-sizing: border-box;
    font-family: var(--sans); font-size: 14px;
    background: transparent; color: var(--ink);
    border: 1px solid var(--rule); border-radius: 4px;
    padding: 9px 12px; outline: none;
  }
  .input:focus { border-color: var(--accent); outline: 2px solid var(--accent); outline-offset: 2px; }
  .hint { font-family: var(--mono); font-size: 11px; color: var(--ink-3); padding: 16px 8px; text-align: center; }
  .hint.err { color: #c43a3a; }
  .results { list-style: none; margin: 12px 0 0; padding: 0; max-height: 50vh; overflow-y: auto; }
  .result {
    display: block; width: 100%; text-align: left;
    padding: 8px 10px; background: transparent; border: 0; cursor: pointer;
    font-family: var(--serif); font-size: 15px; color: var(--ink);
    border-radius: 4px;
  }
  .result:hover { background: var(--bg-soft); }
</style>
