<script lang="ts">
  import type { FilterKey, SortKey } from '../../lib/feedsFilter';

  type Props = {
    search: string;
    filter: FilterKey;
    sort: SortKey;
    counts: Record<FilterKey, number>;
    refreshingAll: boolean;
    onSearch: (q: string) => void;
    onFilter: (k: FilterKey) => void;
    onSort: (s: SortKey) => void;
    onAdd: () => void;
    onRefreshAll: () => void;
    onImport: () => void;
    onExport: () => void;
  };

  let {
    search, filter, sort, counts, refreshingAll,
    onSearch, onFilter, onSort, onAdd, onRefreshAll, onImport, onExport,
  }: Props = $props();

  const chips: { key: FilterKey; label: string }[] = [
    { key: 'all',    label: 'All' },
    { key: 'errors', label: 'Errors' },
    { key: 'unread', label: 'Unread' },
    { key: 'stale',  label: 'Stale' },
  ];
</script>

<div class="ts-feeds-toolbar">
  <!-- Row 1: search + add -->
  <div class="ts-feeds-top">
    <div class="ts-feeds-search">
      <svg class="ts-feeds-search-ico" width="14" height="14" viewBox="0 0 16 16" fill="none" aria-hidden="true">
        <circle cx="6.5" cy="6.5" r="5" stroke="currentColor" stroke-width="1.5"/>
        <line x1="10.5" y1="10.5" x2="14.5" y2="14.5" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/>
      </svg>
      <input
        class="ts-feeds-search-input"
        type="text"
        placeholder="Search feeds by name or URL"
        value={search}
        oninput={(e) => onSearch((e.currentTarget as HTMLInputElement).value)}
      />
    </div>
    <button class="ts-feeds-add btn variant-primary" type="button" onclick={onAdd}>
      <svg width="12" height="12" viewBox="0 0 12 12" fill="none" aria-hidden="true">
        <line x1="6" y1="1" x2="6" y2="11" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/>
        <line x1="1" y1="6" x2="11" y2="6" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/>
      </svg>
      Add feed
    </button>
  </div>

  <!-- Row 2: filter chips + sort + utility buttons -->
  <div class="ts-feeds-row2">
    <div class="ts-feeds-chips">
      {#each chips as chip}
        <button
          type="button"
          class="ts-feeds-chip"
          class:is-active={filter === chip.key}
          class:is-warn={chip.key === 'errors' && counts.errors > 0}
          data-filter-key={chip.key}
          onclick={() => onFilter(chip.key)}
          aria-pressed={filter === chip.key}
          aria-label={chip.label}
        >
          {chip.label}<span class="ct" aria-hidden="true">{counts[chip.key]}</span>
        </button>
      {/each}
    </div>

    <label class="ts-feeds-sort">
      Sort
      <select
        class="ts-feeds-sort-select"
        value={sort}
        onchange={(e) => onSort((e.currentTarget as HTMLSelectElement).value as SortKey)}
      >
        <option value="recent">Recently active</option>
        <option value="name">Name</option>
        <option value="added">Added</option>
        <option value="unread">Most unread</option>
      </select>
    </label>

    <div class="ts-feeds-utils">
      <button
        type="button"
        class="ts-feeds-util-btn"
        class:is-spinning={refreshingAll}
        data-action="refresh-all"
        onclick={onRefreshAll}
        title="Refresh all feeds"
      >
        <svg width="14" height="14" viewBox="0 0 16 16" fill="none" aria-hidden="true">
          <path d="M13.5 2.5A7 7 0 1 0 15 8" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/>
          <polyline points="11,0 14,3 11,6" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/>
        </svg>
        Refresh all
      </button>
      <button type="button" class="ts-feeds-util-btn" onclick={onImport} title="Import OPML">
        <svg width="14" height="14" viewBox="0 0 16 16" fill="none" aria-hidden="true">
          <path d="M8 2v9M5 8l3 3 3-3" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/>
          <path d="M3 13h10" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/>
        </svg>
        Import OPML
      </button>
      <button type="button" class="ts-feeds-util-btn" onclick={onExport} title="Export">
        <svg width="14" height="14" viewBox="0 0 16 16" fill="none" aria-hidden="true">
          <path d="M8 14V5M5 8L8 5l3 3" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/>
          <path d="M3 3h10" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/>
        </svg>
        Export
      </button>
    </div>
  </div>
</div>

<style>
  .ts-feeds-toolbar { display: flex; flex-direction: column; gap: 10px; margin-bottom: 16px; }

  .ts-feeds-top { display: flex; align-items: center; gap: 10px; }
  .ts-feeds-search {
    flex: 1; display: flex; align-items: center; gap: 8px;
    padding: 7px 12px;
    background: var(--bg-soft);
    border: 1px solid var(--rule);
    border-radius: var(--radius-input, 4px);
  }
  .ts-feeds-search-ico { color: var(--ink-3); flex-shrink: 0; }
  .ts-feeds-search-input {
    flex: 1; border: none; background: transparent; outline: none;
    font-family: var(--sans); font-size: 13px; color: var(--ink);
  }
  .ts-feeds-add {
    display: inline-flex; align-items: center; gap: 6px;
    padding: 8px 14px;
    background: var(--ink); color: var(--bg);
    border: 1px solid var(--ink);
    border-radius: var(--radius-input, 4px);
    font-family: var(--sans); font-size: 12.5px; font-weight: 500;
    cursor: pointer; white-space: nowrap;
  }

  .ts-feeds-row2 { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; }
  .ts-feeds-chips { display: flex; gap: 6px; flex-wrap: wrap; }
  .ts-feeds-chip {
    display: inline-flex; align-items: baseline; gap: 6px;
    padding: 4px 10px 4px 11px;
    border: 1px solid var(--rule);
    border-radius: var(--radius-pill, 100px);
    background: var(--bg); color: var(--ink-2);
    font-family: var(--sans); font-size: 12px; font-weight: 500;
    cursor: pointer;
  }
  .ts-feeds-chip:hover { color: var(--ink); border-color: var(--ink-4); }
  .ts-feeds-chip.is-active { background: var(--ink); color: var(--bg); border-color: var(--ink); }
  .ts-feeds-chip.is-warn { color: var(--color-error-light, #c43a3a); border-color: rgba(196,58,58,0.4); }
  :global(html.theme-dark) .ts-feeds-chip.is-warn { color: var(--color-error-dark, #ec7a7a); }
  .ts-feeds-chip .ct { font-family: var(--mono); font-size: 10.5px; color: var(--ink-3); }
  .ts-feeds-chip.is-active .ct { color: var(--bg); opacity: 0.65; }

  .ts-feeds-sort {
    display: flex; align-items: center; gap: 6px;
    font-family: var(--sans); font-size: 12px; color: var(--ink-3);
    margin-left: auto;
  }
  .ts-feeds-sort-select {
    font-family: var(--mono); font-size: 11px; text-transform: uppercase;
    letter-spacing: 0.06em; color: var(--ink-2);
    border: none; background: transparent; cursor: pointer; outline: none;
  }

  .ts-feeds-utils { display: flex; align-items: center; gap: 4px; }
  .ts-feeds-util-btn {
    display: inline-flex; align-items: center; gap: 5px;
    padding: 6px 8px;
    border: none; background: transparent; color: var(--ink-2);
    font-family: var(--sans); font-size: 12px;
    cursor: pointer; border-radius: var(--radius-input, 4px);
  }
  .ts-feeds-util-btn:hover { color: var(--ink); background: var(--bg-soft); }
  @keyframes tf-spin { to { transform: rotate(360deg); } }
  .ts-feeds-util-btn.is-spinning svg { animation: tf-spin 0.8s linear infinite; }
</style>
