<script lang="ts">
  import KbdChip from './KbdChip.svelte';

  type Props = { count: number };
  let { count }: Props = $props();
  const word = $derived(count === 1 ? 'entry' : 'entries');
</script>

<div class="toolbar" role="heading" aria-level={1}>
  <div class="left">
    <span class="eyebrow">Saved</span>
    <span class="count"><b>{count}</b> {word}</span>
  </div>
  <div class="right">
    <span class="find">Find <KbdChip>/</KbdChip></span>
  </div>
</div>

<style>
  .toolbar {
    display: flex; align-items: baseline; justify-content: space-between;
    gap: 16px;
    padding: 20px 2px 14px;
    border-bottom: 1px solid var(--rule);
    margin-bottom: 4px;
  }
  .left { display: flex; align-items: baseline; gap: 14px; flex-wrap: wrap; }
  .eyebrow {
    font-family: var(--serif); font-size: 26px; font-weight: 600;
    letter-spacing: -0.02em; color: var(--ink);
  }
  .count {
    font-family: var(--mono); font-size: 11px;
    color: var(--ink-3); letter-spacing: 0.04em;
  }
  .count b { color: var(--ink); font-weight: 500; }
  .right { display: flex; align-items: center; gap: 14px; flex-shrink: 0; }
  .find {
    font-family: var(--mono); font-size: 11px;
    letter-spacing: 0.04em; color: var(--ink-3);
    display: inline-flex; align-items: center; gap: 6px;
  }
</style>
