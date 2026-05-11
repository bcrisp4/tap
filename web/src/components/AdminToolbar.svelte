<script lang="ts">
  export type AdminFilter = 'all' | 'admins' | 'users' | 'disabled';
  type Props = {
    filter: AdminFilter;
    query: string;
    onFilter?: (f: AdminFilter) => void;
    onQuery?: (q: string) => void;
    onCreate?: () => void;
  };
  const { filter, query, onFilter, onQuery, onCreate }: Props = $props();
  const chips: Array<{ id: AdminFilter; label: string }> = [
    { id: 'all', label: 'All' },
    { id: 'admins', label: 'Admins' },
    { id: 'users', label: 'Users' },
    { id: 'disabled', label: 'Disabled' },
  ];
</script>

<div class="bar">
  <div class="left">
    <label class="search">
      <span class="ico" aria-hidden="true">
        <svg width="12" height="12" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"><circle cx="7" cy="7" r="4.5"/><path d="m10.5 10.5 3 3"/></svg>
      </span>
      <input
        type="search"
        placeholder="Filter by username"
        value={query}
        oninput={(e) => onQuery?.((e.currentTarget as HTMLInputElement).value)}
      />
    </label>
    <div class="chips" role="group" aria-label="Filter users">
      {#each chips as c (c.id)}
        <button type="button" class="chip" class:active={filter === c.id} aria-pressed={filter === c.id} onclick={() => onFilter?.(c.id)}>{c.label}</button>
      {/each}
    </div>
  </div>
  <button type="button" class="create" onclick={() => onCreate?.()}>
    <svg width="12" height="12" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"><path d="M8 3.5v9M3.5 8h9"/></svg>
    <span>Create user</span>
  </button>
</div>

<style>
  .bar { display: flex; align-items: center; justify-content: space-between; gap: 14px; margin-bottom: 14px; }
  .left { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; }
  .search {
    display: inline-flex; align-items: center; gap: 8px;
    padding: 6px 10px;
    border: 1px solid var(--rule);
    border-radius: 4px;
    background: var(--bg);
    min-width: 240px;
  }
  .search .ico { color: var(--ink-3); display: inline-flex; }
  .search input {
    font-family: var(--sans); font-size: 13px; color: var(--ink);
    border: 0; background: transparent; outline: none; flex: 1;
  }
  .search input::placeholder { color: var(--ink-3); }
  .chips { display: inline-flex; gap: 4px; }
  .chip {
    font-family: var(--mono); font-size: 10.5px; letter-spacing: 0.04em;
    color: var(--ink-2);
    padding: 5px 10px;
    border: 1px solid var(--rule);
    background: var(--bg);
    border-radius: 999px;
    cursor: pointer;
  }
  .chip:hover { color: var(--ink); border-color: var(--ink-4); }
  .chip.active { color: var(--accent); border-color: var(--accent); background: var(--accent-soft); }
  .create {
    display: inline-flex; align-items: center; gap: 6px;
    font-family: var(--sans); font-size: 13px; font-weight: 500;
    color: var(--bg);
    background: var(--ink);
    border: 1px solid var(--ink);
    border-radius: 4px;
    padding: 7px 12px;
    cursor: pointer;
  }
  .create:hover { background: var(--ink-2); border-color: var(--ink-2); }
</style>
