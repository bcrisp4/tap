<script lang="ts">
  type Props = { label: string; count?: number };
  let { label, count }: Props = $props();
</script>

<div class="heading">
  <span class="label">{label}</span>
  <span class="rule" aria-hidden="true"></span>
  {#if count !== undefined}<span class="count">{count}</span>{/if}
</div>

<style>
  .heading {
    display: flex; align-items: center; gap: 12px;
    padding: 28px 2px 10px;
    font-family: var(--mono); font-size: 10px;
    letter-spacing: 0.14em; text-transform: uppercase;
    color: var(--ink-3);
  }
  .heading:first-child { padding-top: 18px; }
  .label { flex-shrink: 0; }
  .rule { flex: 1; height: 1px; background: var(--rule); }
  .count { font-family: var(--mono); font-size: 10px; color: var(--ink-3); letter-spacing: 0.04em; }
</style>
