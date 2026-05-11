<script lang="ts">
  interface Props {
    label: string;
    desc?: string;
    stacked?: boolean;
    block?: boolean;
    control?: import('svelte').Snippet;
    children?: import('svelte').Snippet;
  }
  let { label, desc, stacked = false, block = false, control, children }: Props = $props();
</script>

<div class="set-row" class:is-stacked={stacked} class:is-block={block}>
  <div>
    <div class="set-label">{label}</div>
    {#if desc}<div class="set-desc">{desc}</div>{/if}
    {#if block && children}{@render children()}{/if}
  </div>
  {#if !block && control}{@render control()}{/if}
</div>

<style>
  .set-row {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
    gap: 24px;
    align-items: center;
    padding: 14px 2px;
    border-bottom: 1px solid var(--rule);
  }
  .set-row.is-stacked {
    grid-template-columns: 1fr;
    gap: 14px;
  }
  .set-row.is-block {
    grid-template-columns: 1fr;
    align-items: stretch;
    padding: 18px 2px;
    gap: 12px;
  }
  .set-row:last-child { border-bottom: 0; }
  .set-label {
    font-family: var(--serif);
    font-size: 17px;
    font-weight: 500;
    color: var(--ink);
    letter-spacing: -0.005em;
    line-height: 1.3;
  }
  .set-desc {
    font-family: var(--sans);
    font-size: 12.5px;
    color: var(--ink-2);
    margin-top: 4px;
    line-height: 1.5;
    max-width: 440px;
  }
  @media (max-width: 540px) {
    .set-row { grid-template-columns: 1fr; gap: 14px; }
  }
</style>
