<script lang="ts">
  type Variant = 'default' | 'primary' | 'accent' | 'danger' | 'quiet';
  type Size = 'sm' | 'md';
  type Props = {
    variant?: Variant;
    size?: Size;
    type?: 'button' | 'submit';
    disabled?: boolean;
    title?: string;
    onclick?: (e: MouseEvent) => void;
    children: import('svelte').Snippet;
  };
  let { variant = 'default', size = 'md', type = 'button', disabled = false, title, onclick, children }: Props = $props();
</script>

<button
  {type}
  {title}
  {disabled}
  class="btn variant-{variant} size-{size}"
  {onclick}
>
  {@render children()}
</button>

<style>
  .btn {
    display: inline-flex; align-items: center; gap: 7px;
    font-family: var(--sans);
    font-size: var(--fs-button, 12.5px);
    font-weight: 500;
    padding: 8px 14px;
    border-radius: var(--radius-input, 4px);
    border: 1px solid var(--rule);
    background: var(--bg);
    color: var(--ink);
    cursor: pointer;
    white-space: nowrap;
    transition: background var(--dur-fast, 100ms) ease, border-color var(--dur-fast, 100ms) ease;
  }
  .btn:hover { background: var(--bg-soft); border-color: var(--ink-4); }
  .btn:disabled { opacity: 0.5; cursor: not-allowed; }
  .btn.variant-primary { background: var(--ink); color: var(--bg); border-color: var(--ink); }
  .btn.variant-primary:hover { opacity: 0.88; }
  .btn.variant-accent { color: var(--accent); border-color: var(--accent); background: transparent; }
  .btn.variant-accent:hover { background: var(--accent-soft); }
  .btn.variant-danger { color: var(--color-error-light, #c43a3a); border-color: rgba(196, 58, 58, 0.4); background: transparent; }
  :global(html.theme-dark) .btn.variant-danger { color: var(--color-error-dark, #ec7a7a); border-color: rgba(236, 122, 122, 0.4); }
  .btn.variant-danger:hover { background: rgba(196, 58, 58, 0.06); }
  .btn.variant-quiet { border-color: transparent; background: transparent; color: var(--ink-2); padding: 6px 8px; }
  .btn.variant-quiet:hover { color: var(--ink); background: var(--bg-soft); }
  .btn.size-sm { padding: 6px 10px; font-size: 11px; }
</style>
