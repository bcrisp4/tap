<script lang="ts">
  import Popover from './Popover.svelte';
  import type { Category } from '../lib/types';

  type Props = {
    open: boolean;
    anchor: HTMLElement | null;
    feedName: string;
    currentCategoryId: number | null;
    categories: Category[];
    label?: 'Move' | 'Assign';
    onPick: (id: number | null) => void;
    onClose: () => void;
  };
  const { open, anchor, feedName, currentCategoryId, categories, label = 'Move', onPick, onClose }: Props = $props();

  let top = $state(0);
  let left = $state(0);

  $effect(() => {
    if (!open || !anchor) return;
    function reposition() {
      if (!anchor) return;
      const r = anchor.getBoundingClientRect();
      top = r.bottom + 4;
      left = r.left;
    }
    reposition();
    window.addEventListener('resize', reposition);
    return () => {
      window.removeEventListener('resize', reposition);
    };
  });
</script>

<Popover {open} anchor="free" {onClose}>
  <div class="ts-cat-pop" aria-label="{label} {feedName}" style:top="{top}px" style:left="{left}px">
    <div class="ts-cat-pop-eyebrow">{label} <b>{feedName}</b> to</div>
    <div class="ts-cat-pop-rule"></div>
    {#each categories as cat (cat.id)}
      <button
        class="ts-cat-pop-item"
        class:is-current={cat.id === currentCategoryId}
        onclick={() => onPick(cat.id)}
        role="menuitem"
      >
        <span class="check" aria-hidden="true">✓</span>
        <span>{cat.name}</span>
        <span class="ts-cat-pop-item-ct">{cat.unread}</span>
      </button>
    {/each}
    <div class="ts-cat-pop-rule"></div>
    <button
      class="ts-cat-pop-item is-uncat"
      class:is-current={currentCategoryId === null}
      onclick={() => onPick(null)}
      role="menuitem"
    >
      <span class="check" aria-hidden="true">✓</span>
      <span>Uncategorised</span>
      <span class="ts-cat-pop-item-ct"></span>
    </button>
  </div>
</Popover>

<style>
  .ts-cat-pop {
    position: fixed;
    min-width: 220px;
    background: var(--bg);
    border: 1px solid var(--rule);
    border-radius: 5px;
    box-shadow: 0 16px 40px rgba(0, 0, 0, 0.18);
    padding: 4px;
  }
  :global(html.theme-dark) .ts-cat-pop { box-shadow: 0 20px 50px rgba(0, 0, 0, 0.55); }
  .ts-cat-pop-eyebrow {
    font-family: var(--mono); font-size: 9.5px; letter-spacing: 0.12em;
    text-transform: uppercase; color: var(--ink-3);
    padding: 8px 10px 6px; display: flex; gap: 6px;
  }
  .ts-cat-pop-eyebrow b {
    color: var(--ink); font-weight: 500; text-transform: none; letter-spacing: 0.02em;
  }
  .ts-cat-pop-rule { height: 1px; background: var(--rule); margin: 2px 4px; }
  .ts-cat-pop-item {
    display: flex; align-items: center; gap: 8px; width: 100%;
    padding: 7px 10px; border: 0; background: transparent;
    text-align: left;
    font-family: var(--sans); font-size: 13px; font-weight: 500;
    color: var(--ink); cursor: pointer; border-radius: 3px;
  }
  .ts-cat-pop-item:hover { background: var(--bg-soft); }
  .ts-cat-pop-item .check {
    width: 10px; display: inline-flex; color: var(--accent);
    font-size: 11px; visibility: hidden;
  }
  .ts-cat-pop-item.is-current { color: var(--accent); }
  .ts-cat-pop-item.is-current .check { visibility: visible; }
  .ts-cat-pop-item.is-uncat { color: var(--ink-3); font-style: italic; font-weight: 400; }
  .ts-cat-pop-item-ct {
    margin-left: auto;
    font-family: var(--mono); font-size: 10.5px;
    color: var(--ink-4); font-style: normal; font-weight: 400;
  }
</style>
