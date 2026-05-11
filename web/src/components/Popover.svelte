<script lang="ts">
  type Props = {
    open: boolean;
    onClose: () => void;
    anchor?: 'top-right' | 'bottom-left' | 'bottom-right' | 'free';
    children: import('svelte').Snippet;
  };
  let { open, onClose, anchor = 'free', children }: Props = $props();

  $effect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose();
    };
    document.addEventListener('keydown', onKey);
    return () => document.removeEventListener('keydown', onKey);
  });
</script>

{#if open}
  <div class="scrim" onclick={onClose} role="presentation"></div>
  <div class="popover anchor-{anchor}" role="menu" onclick={(e) => e.stopPropagation()}>
    {@render children()}
  </div>
{/if}

<style>
  .scrim { position: fixed; inset: 0; z-index: 150; }
  .popover {
    position: absolute; z-index: 151;
    background: var(--bg);
    border: 1px solid var(--rule);
    border-radius: 6px;
    box-shadow: var(--shadow-popover, 0 10px 30px rgba(0,0,0,0.18));
    padding: 4px;
    font-family: var(--sans);
    min-width: 200px;
  }
  .anchor-top-right { right: 18px; top: 56px; }
  .anchor-bottom-left { left: 14px; bottom: 56px; }
  .anchor-bottom-right { right: 18px; bottom: 56px; }
</style>
