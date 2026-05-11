<script lang="ts">
  type Props = {
    open: boolean;
    onClose: () => void;
    title: string;
    wide?: boolean;
    foot?: import('svelte').Snippet;
    children: import('svelte').Snippet;
  };
  let { open, onClose, title, wide = false, foot, children }: Props = $props();

  $effect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => { if (e.key === 'Escape') onClose(); };
    document.addEventListener('keydown', onKey);
    return () => document.removeEventListener('keydown', onKey);
  });
</script>

{#if open}
  <div class="scrim" onclick={onClose} role="presentation"></div>
  <div
    class="dialog"
    class:is-wide={wide}
    role="dialog"
    aria-modal="true"
    aria-label={title}
    onclick={(e) => e.stopPropagation()}
  >
    <div class="head">
      <span class="title">{title}</span>
      <button class="close" onclick={onClose} aria-label="Close" type="button">
        <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"><path d="M3.5 3.5l9 9M12.5 3.5l-9 9"/></svg>
      </button>
    </div>
    <div class="body">{@render children()}</div>
    {#if foot}<div class="foot">{@render foot()}</div>{/if}
  </div>
{/if}

<style>
  .scrim {
    position: fixed; inset: 0; z-index: 200;
    background: rgba(0,0,0,0.42);
    backdrop-filter: blur(2px);
  }
  :global(html.theme-dark) .scrim { background: rgba(0,0,0,0.6); }
  .dialog {
    position: fixed;
    left: 50%; top: 64px;
    transform: translateX(-50%);
    width: min(520px, calc(100% - 32px));
    max-height: calc(100vh - 96px);
    overflow: auto;
    z-index: 201;
    background: var(--bg);
    border: 1px solid var(--rule);
    border-radius: var(--radius-dialog, 6px);
    box-shadow: var(--shadow-dialog, 0 24px 60px rgba(0,0,0,0.32));
  }
  .dialog.is-wide { width: min(640px, 92vw); }
  .head {
    display: flex; align-items: center; justify-content: space-between;
    padding: 16px 20px 14px;
    border-bottom: 1px solid var(--rule);
  }
  .title { font-family: var(--serif); font-size: 18px; font-weight: 600; color: var(--ink); letter-spacing: -0.01em; }
  .close {
    width: 28px; height: 28px;
    display: inline-flex; align-items: center; justify-content: center;
    border: 0; background: transparent;
    color: var(--ink-3); cursor: pointer; border-radius: 4px;
  }
  .close:hover { color: var(--ink); background: var(--bg-soft); }
  .body { padding: 18px 20px; }
  .foot {
    display: flex; justify-content: flex-end; gap: 8px;
    padding: 14px 20px;
    border-top: 1px solid var(--rule);
    background: var(--bg-soft);
  }
</style>
