<script lang="ts">
  import { searchOverlay } from '../lib/searchOverlay.svelte';
</script>

{#if searchOverlay.open}
  <div class="search-overlay" role="dialog" aria-label="Search">
    <div class="scrim" onclick={() => searchOverlay.close()} role="presentation"></div>
    <div class="panel">
      <input
        class="input"
        type="search"
        aria-label="Search entries"
        autofocus
        placeholder="Search (M2 will implement)"
        value={searchOverlay.query}
        oninput={(e) => searchOverlay.setQuery((e.currentTarget as HTMLInputElement).value)}
      />
      <p class="hint">M2 will land the actual filter behaviour.</p>
    </div>
  </div>
{/if}

<style>
  .search-overlay { position: fixed; inset: 0; z-index: 220; display: flex; align-items: flex-start; justify-content: center; padding-top: 64px; }
  .scrim { position: absolute; inset: 0; background: rgba(0,0,0,0.42); backdrop-filter: blur(2px); }
  .panel {
    position: relative;
    background: var(--bg);
    border: 1px solid var(--rule);
    border-radius: 6px;
    padding: 14px 16px;
    width: min(520px, calc(100% - 32px));
    box-shadow: var(--shadow-dialog, 0 24px 60px rgba(0,0,0,0.32));
  }
  .input {
    width: 100%;
    font-family: var(--sans); font-size: 14px;
    padding: 9px 12px;
    border: 1px solid var(--rule);
    border-radius: 4px;
    background: var(--bg-soft);
    color: var(--ink);
  }
  .input:focus { outline: 2px solid var(--accent); outline-offset: 1px; border-color: var(--accent); }
  .hint { font-family: var(--mono); font-size: 10.5px; color: var(--ink-3); margin: 10px 4px 0; letter-spacing: 0.04em; }
</style>
