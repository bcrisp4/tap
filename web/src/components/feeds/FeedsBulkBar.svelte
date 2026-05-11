<script lang="ts">
  type Props = {
    count: number;
    onClear: () => void;
    onRefresh: () => void;
    onReassign: (anchor: HTMLElement) => void;
    onDelete: () => void;
  };
  let { count, onClear, onRefresh, onReassign, onDelete }: Props = $props();
</script>

<div class="ts-feeds-bulk">
  <span class="ts-feeds-bulk-count">{count} selected</span>
  <button type="button" class="ts-feeds-bulk-clear" onclick={onClear}>Clear</button>
  <span class="ts-feeds-bulk-spacer"></span>
  <button type="button" class="ts-feeds-bulk-btn" onclick={onRefresh}>
    <svg width="12" height="12" viewBox="0 0 16 16" fill="none" aria-hidden="true">
      <path d="M13.5 2.5A7 7 0 1 0 15 8" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/>
      <polyline points="11,0 14,3 11,6" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/>
    </svg>
    Refresh
  </button>
  <button
    type="button"
    class="ts-feeds-bulk-btn"
    onclick={(e) => onReassign(e.currentTarget as HTMLElement)}
  >
    Set category
    <svg width="10" height="10" viewBox="0 0 10 10" fill="none" aria-hidden="true">
      <polyline points="2,3 5,7 8,3" stroke="currentColor" stroke-width="1.3" stroke-linecap="round" stroke-linejoin="round"/>
    </svg>
  </button>
  <button type="button" class="ts-feeds-bulk-btn is-danger" onclick={onDelete}>
    <svg width="12" height="12" viewBox="0 0 16 16" fill="none" aria-hidden="true">
      <polyline points="2,4 14,4" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"/>
      <path d="M6 4V2h4v2M5 4v9h6V4" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round"/>
    </svg>
    Delete
  </button>
</div>

<style>
  .ts-feeds-bulk {
    display: flex; align-items: center; gap: 8px;
    padding: 8px 12px;
    background: var(--ink);
    border-radius: var(--radius-input, 4px);
    margin-bottom: 12px;
  }
  .ts-feeds-bulk-count { font-family: var(--mono); font-size: 11.5px; color: var(--bg); opacity: 0.7; }
  .ts-feeds-bulk-clear {
    font-family: var(--sans); font-size: 11.5px; color: var(--bg); opacity: 0.6;
    border: none; background: transparent; cursor: pointer; padding: 2px 4px;
    border-radius: 3px;
  }
  .ts-feeds-bulk-clear:hover { opacity: 1; }
  .ts-feeds-bulk-spacer { flex: 1; }
  .ts-feeds-bulk-btn {
    display: inline-flex; align-items: center; gap: 5px;
    padding: 5px 10px;
    border: 1px solid rgba(255,255,255,0.25);
    border-radius: var(--radius-input, 4px);
    background: transparent; color: var(--bg);
    font-family: var(--sans); font-size: 12px;
    cursor: pointer;
  }
  .ts-feeds-bulk-btn:hover { background: rgba(255,255,255,0.12); }
  .ts-feeds-bulk-btn.is-danger { color: var(--color-error-dark, #ec7a7a); border-color: rgba(236,122,122,0.3); }
</style>
