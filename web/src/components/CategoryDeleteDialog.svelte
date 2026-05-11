<script lang="ts">
  import Dialog from './Dialog.svelte';
  import type { Subscription } from '../lib/types';

  type Props = {
    categoryName: string;
    affectedFeeds: Subscription[];
    onCancel: () => void;
    onConfirm: () => void;
  };
  const { categoryName, affectedFeeds, onCancel, onConfirm }: Props = $props();

  const shown = $derived(affectedFeeds.slice(0, 5));
  const extra = $derived(Math.max(0, affectedFeeds.length - 5));
</script>

<Dialog open={true} title={`Delete "${categoryName}"?`} onClose={onCancel}>
  <p class="ts-dialog-p">
    The category is removed.
    {#if affectedFeeds.length > 0}
      The {affectedFeeds.length} {affectedFeeds.length === 1 ? 'feed' : 'feeds'} inside stay subscribed —
      they drop back to <b>Uncategorised</b> and keep appearing in Unread.
    {:else}
      No feeds are assigned to it, so nothing else changes.
    {/if}
  </p>
  {#if affectedFeeds.length > 0}
    <ul class="ts-dialog-list">
      {#each shown as f (f.id)}
        <li class="ts-dialog-list-item">
          <span class="ts-dialog-list-item-name">{f.title}</span>
          <span class="ts-dialog-list-item-arrow">&rarr; Uncategorised</span>
        </li>
      {/each}
      {#if extra > 0}
        <li class="ts-dialog-list-more">+ {extra} more</li>
      {/if}
    </ul>
  {/if}
  {#snippet foot()}
    <div class="ts-dialog-foot-l">esc to cancel</div>
    <button class="ts-btn" onclick={onCancel}>Cancel</button>
    <button class="ts-btn is-danger" onclick={onConfirm}>Delete category</button>
  {/snippet}
</Dialog>

<style>
  .ts-dialog-p {
    font-family: var(--serif); font-size: 15px; line-height: 1.55;
    color: var(--ink-2); margin: 0;
  }
  .ts-dialog-list {
    list-style: none; margin: 14px 0 0; padding: 12px 0 0;
    border-top: 1px solid var(--rule);
  }
  .ts-dialog-list-item {
    display: flex; align-items: center; gap: 10px; padding: 6px 0;
    font-family: var(--sans); font-size: 13px; color: var(--ink-2);
  }
  .ts-dialog-list-item-name { color: var(--ink); font-weight: 500; }
  .ts-dialog-list-item-arrow {
    margin-left: auto; font-family: var(--mono); font-size: 10.5px; color: var(--ink-3);
  }
  .ts-dialog-list-more {
    font-family: var(--mono); font-size: 10.5px; color: var(--ink-4);
    padding: 6px 0 0; letter-spacing: 0.04em;
  }
  .ts-dialog-foot-l {
    font-family: var(--mono); font-size: 10.5px; color: var(--ink-3);
    letter-spacing: 0.06em; margin-right: auto;
  }
</style>
