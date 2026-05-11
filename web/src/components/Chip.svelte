<script lang="ts">
  type Variant = 'pill' | 'tag' | 'error' | 'backoff';
  type Props = {
    variant?: Variant;
    active?: boolean;
    count?: number;
    onclick?: () => void;
    children: import('svelte').Snippet;
  };
  let { variant = 'pill', active = false, count, onclick, children }: Props = $props();
</script>

<button
  type="button"
  class="chip variant-{variant}"
  class:is-active={active}
  {onclick}
>
  {@render children()}
  {#if count !== undefined}<span class="ct">{count}</span>{/if}
</button>

<style>
  .chip {
    display: inline-flex; align-items: baseline; gap: 8px;
    padding: 5px 10px 5px 12px;
    border: 1px solid var(--rule);
    border-radius: var(--radius-pill, 100px);
    background: var(--bg);
    color: var(--ink-2);
    font-family: var(--sans);
    font-size: 12px; font-weight: 500;
    cursor: pointer;
    transition: all var(--dur-base, 120ms) ease;
  }
  .chip:hover { color: var(--ink); border-color: var(--ink-4); }
  .chip.is-active { background: var(--ink); color: var(--bg); border-color: var(--ink); }
  .chip .ct { font-family: var(--mono); font-size: 10.5px; color: var(--ink-3); letter-spacing: 0; }
  .chip.is-active .ct { color: var(--bg); opacity: 0.65; }
  .variant-tag { background: var(--bg-soft); border-color: transparent; padding: 3px 8px; font-size: 11.5px; }
  .variant-error { font-family: var(--mono); font-size: 9.5px; padding: 2px 8px; color: var(--color-error-light, #c43a3a); border-color: rgba(196, 58, 58, 0.4); text-transform: uppercase; letter-spacing: 0.08em; }
  :global(html.theme-dark) .variant-error { color: var(--color-error-dark, #ec7a7a); border-color: rgba(236, 122, 122, 0.4); }
  .variant-backoff { font-family: var(--mono); font-size: 9.5px; padding: 2px 8px; color: var(--ink-3); border-style: dashed; border-color: var(--ink-4); text-transform: uppercase; letter-spacing: 0.08em; }
</style>
