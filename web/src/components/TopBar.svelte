<script lang="ts">
  type Props = {
    title: string;
    countShown: number;
    countTotal: number;
    onRefresh?: () => void;
  };
  let { title, countShown, countTotal, onRefresh }: Props = $props();
</script>

<header class="topbar">
  <span class="crumb">{title}</span>
  <span class="count">{countShown} of {countTotal}</span>
  <span class="spacer"></span>
  {#if onRefresh}
    <button class="icon" onclick={onRefresh} aria-label="Refresh">↻</button>
  {/if}
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
  .icon {
    width: 28px;
    height: 28px;
    border-radius: 4px;
    color: var(--ink-2);
  }
  .icon:hover { background: var(--bg-soft); }
</style>
