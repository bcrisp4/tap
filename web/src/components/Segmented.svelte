<script lang="ts" generics="T extends string">
  type Option = { value: T; label: string; preview?: import('svelte').Snippet };
  type Props = {
    options: Option[];
    value: T;
    onChange: (v: T) => void;
    ariaLabel?: string;
  };
  let { options, value, onChange, ariaLabel }: Props = $props();
</script>

<div class="segmented" role="radiogroup" aria-label={ariaLabel}>
  {#each options as o (o.value)}
    <button
      type="button"
      role="radio"
      aria-checked={value === o.value}
      class="btn"
      class:is-active={value === o.value}
      onclick={() => onChange(o.value)}
    >
      {#if o.preview}{@render o.preview()}{/if}
      <span>{o.label}</span>
    </button>
  {/each}
</div>

<style>
  .segmented {
    display: inline-flex; align-items: stretch;
    background: var(--bg-soft);
    border: 1px solid var(--rule);
    border-radius: 999px;
    padding: 3px; gap: 2px;
  }
  .btn {
    display: inline-flex; align-items: center; gap: 7px;
    padding: 7px 14px;
    border: 0; background: transparent;
    border-radius: 999px;
    font-family: var(--sans); font-size: 12.5px; font-weight: 500;
    color: var(--ink-2); cursor: pointer;
    white-space: nowrap;
    transition: background var(--dur-fast, 100ms) ease, color var(--dur-fast, 100ms) ease;
  }
  .btn:hover { color: var(--ink); }
  .btn.is-active {
    background: var(--bg); color: var(--ink);
    box-shadow: 0 1px 2px rgba(0,0,0,0.06), 0 0 0 1px var(--rule);
  }
</style>
