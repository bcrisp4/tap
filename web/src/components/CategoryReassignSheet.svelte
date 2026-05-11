<script lang="ts">
  import type { Category } from '../lib/types';

  type Props = {
    open: boolean;
    feedName: string;
    currentCategoryId: number | null;
    categories: Category[];
    onSelect: (id: number | null) => void;
    onClose: () => void;
  };
  const { open, feedName, currentCategoryId, categories, onSelect, onClose }: Props = $props();

  function pick(id: number | null) {
    onSelect(id);
    onClose();
  }
</script>

{#if open}
  <div
    class="m-cat-sheet"
    role="dialog"
    aria-modal="true"
    aria-label="Move feed"
    onclick={onClose}
  >
    <div class="m-cat-sheet-card" onclick={(e) => e.stopPropagation()}>
      <div class="m-cat-sheet-handle" aria-hidden="true"></div>
      <div class="m-cat-sheet-head">
        <div class="m-cat-sheet-eyebrow">Move feed</div>
        <div class="m-cat-sheet-title"><span>{feedName}</span></div>
      </div>
      <div class="m-cat-sheet-list">
        {#each categories as cat (cat.id)}
          <button
            class="m-cat-sheet-item"
            class:is-current={cat.id === currentCategoryId}
            onclick={() => pick(cat.id)}
          >
            <span class="m-cat-sheet-item-check">✓</span>
            <span>{cat.name}</span>
          </button>
        {/each}
        <button
          class="m-cat-sheet-item is-uncat"
          class:is-current={currentCategoryId === null}
          onclick={() => pick(null)}
        >
          <span class="m-cat-sheet-item-check">✓</span>
          <span>Uncategorised</span>
        </button>
      </div>
      <div class="m-cat-sheet-foot">
        <span></span>
        <button class="m-cat-sheet-cancel" onclick={onClose}>Cancel</button>
      </div>
    </div>
  </div>
{/if}

<style>
  .m-cat-sheet {
    position: fixed; inset: 0;
    background: rgba(0, 0, 0, 0.42);
    display: flex; align-items: flex-end; justify-content: stretch;
    z-index: 60;
  }
  :global(html.theme-dark) .m-cat-sheet { background: rgba(0, 0, 0, 0.62); }
  .m-cat-sheet-card {
    flex: 1; background: var(--bg);
    border-radius: 16px 16px 0 0;
    padding: 10px 0 calc(env(safe-area-inset-bottom, 0px) + 18px);
    max-height: 78%;
    display: flex; flex-direction: column;
  }
  .m-cat-sheet-handle {
    width: 38px; height: 4px; background: var(--ink-4);
    border-radius: 2px; margin: 0 auto 10px; opacity: 0.5;
  }
  .m-cat-sheet-head { padding: 8px 22px 14px; border-bottom: 1px solid var(--rule); }
  .m-cat-sheet-eyebrow {
    font-family: var(--mono); font-size: 10px; letter-spacing: 0.12em;
    text-transform: uppercase; color: var(--ink-3);
  }
  .m-cat-sheet-title {
    font-family: var(--serif); font-size: 20px; font-weight: 600;
    letter-spacing: -0.01em; color: var(--ink);
    margin-top: 4px; display: flex; align-items: center; gap: 8px;
  }
  .m-cat-sheet-list { flex: 1; overflow-y: auto; padding: 6px 0; }
  .m-cat-sheet-item {
    display: flex; align-items: center; width: 100%;
    padding: 16px 22px; background: transparent; border: 0;
    border-bottom: 1px solid var(--rule);
    font-family: var(--serif); font-size: 18px; font-weight: 500;
    color: var(--ink); letter-spacing: -0.005em;
    text-align: left; cursor: pointer; gap: 12px;
  }
  .m-cat-sheet-item:last-child { border-bottom: 0; }
  .m-cat-sheet-item.is-current { color: var(--accent); }
  .m-cat-sheet-item.is-uncat { color: var(--ink-3); font-style: italic; font-weight: 400; }
  .m-cat-sheet-item-check {
    width: 14px; color: var(--accent);
    visibility: hidden; text-align: center;
  }
  .m-cat-sheet-item.is-current .m-cat-sheet-item-check { visibility: visible; }
  .m-cat-sheet-foot {
    padding: 14px 22px 4px; border-top: 1px solid var(--rule);
    display: flex; justify-content: space-between; align-items: center;
  }
  .m-cat-sheet-cancel {
    font-family: var(--sans); font-size: 14px; font-weight: 500;
    color: var(--ink-2); background: transparent; border: 0;
    padding: 8px 4px; cursor: pointer;
  }
</style>
