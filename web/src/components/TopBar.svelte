<script lang="ts">
  import { navigate } from '../lib/router';

  type Props = {
    title: string;
    countShown: number;
    countTotal: number;
    onRefresh?: () => void;
    onMarkAllRead?: () => void;
    onSearch?: () => void;
  };
  let { title, countShown, countTotal, onRefresh, onMarkAllRead, onSearch }: Props = $props();
</script>

<header class="topbar">
  <span class="crumb">{title}</span>
  <span class="count">{countShown} of {countTotal}</span>
  <span class="spacer"></span>
  <div class="actions">
    <button class="icon" onclick={() => onSearch ? onSearch() : navigate('/search')} aria-label="Search">🔍</button>
    {#if onMarkAllRead}
      <button class="icon" onclick={onMarkAllRead} aria-label="Mark all as read">✓</button>
    {/if}
    {#if onRefresh}
      <button class="icon" onclick={onRefresh} aria-label="Refresh feeds">↻</button>
    {/if}
  </div>
</header>

<style>
  .topbar {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 14px 24px;
    border-bottom: 1px solid var(--rule);
    background: var(--bg);
  }
  .crumb {
    font-family: var(--sans);
    font-weight: 600;
    font-size: 13px;
  }
  .count {
    font-family: var(--mono);
    font-size: 11px;
    color: var(--ink-3);
    margin-left: 8px;
  }
  .spacer { flex: 1; }
  .actions { display: flex; gap: 4px; }
  .icon {
    width: 28px;
    height: 28px;
    border-radius: 4px;
    color: var(--ink-2);
  }
  .icon:hover { background: var(--bg-soft); }
</style>
