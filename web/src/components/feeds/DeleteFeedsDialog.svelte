<script lang="ts">
  import Dialog from '../Dialog.svelte';
  import FeedAvatar from '../FeedAvatar.svelte';
  import { displayUrl } from '../../lib/url';
  import type { Subscription } from '../../lib/types';

  type Props = {
    feeds: Subscription[];
    onClose: () => void;
    onConfirm: () => Promise<void>;
  };
  let { feeds, onClose, onConfirm }: Props = $props();

  let busy = $state(false);
  let error = $state<string | null>(null);

  const title = $derived(
    feeds.length === 1 ? `Delete "${feeds[0].title}"?` : `Delete ${feeds.length} feeds?`
  );
  const label = $derived(feeds.length === 1 ? 'Delete feed' : `Delete ${feeds.length} feeds`);
  const shown = $derived(feeds.slice(0, 5));
  const overflow = $derived(feeds.length > 5 ? feeds.length - 5 : 0);

  async function confirm() {
    busy = true;
    error = null;
    try {
      await onConfirm();
    } catch (e) {
      error = (e as Error).message;
    } finally {
      busy = false;
    }
  }
</script>

<Dialog open={true} {title} {onClose}>
  {#snippet foot()}
    <div class="ts-delete-foot">
      <button type="button" class="btn-cancel" onclick={onClose}>Cancel</button>
      <button
        type="button"
        class="btn-confirm"
        disabled={busy}
        onclick={confirm}
        aria-label={label}
      >{label}</button>
    </div>
  {/snippet}

  <div class="ts-delete-body">
    <p class="ts-delete-warn">
      This action cannot be undone. {feeds.length === 1 ? 'The feed and all its entries will be removed.' : 'All selected feeds and their entries will be removed.'}
    </p>
    <ul class="ts-dialog-list">
      {#each shown as f}
        <li class="ts-dialog-list-item">
          <FeedAvatar feedURL={f.feed_url} size={14} />
          <span class="item-title">{f.title}</span>
          <span class="item-url">{displayUrl(f.feed_url)}</span>
        </li>
      {/each}
      {#if overflow > 0}
        <li class="ts-dialog-list-more">…and {overflow} more</li>
      {/if}
    </ul>
    {#if error}
      <p class="ts-delete-error">{error}</p>
    {/if}
  </div>
</Dialog>

<style>
  .ts-delete-body { display: flex; flex-direction: column; gap: 12px; }
  .ts-delete-warn { font-family: var(--sans); font-size: 13px; color: var(--ink-2); margin: 0; }
  .ts-dialog-list { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: 6px; }
  .ts-dialog-list-item { display: flex; align-items: center; gap: 10px; padding: 6px 10px; background: var(--bg-soft); border-radius: 4px; }
  .item-title { font-family: var(--sans); font-size: 13px; font-weight: 500; color: var(--ink); flex: 1; }
  .item-url { font-family: var(--mono); font-size: 11px; color: var(--ink-3); }
  .ts-dialog-list-more { font-family: var(--sans); font-size: 12px; color: var(--ink-3); padding: 4px 10px; }
  .ts-delete-error { font-family: var(--sans); font-size: 12.5px; color: var(--color-error-light, #c43a3a); margin: 0; }
  :global(html.theme-dark) .ts-delete-error { color: var(--color-error-dark, #ec7a7a); }
  .ts-delete-foot { display: flex; justify-content: flex-end; gap: 8px; }
  .btn-cancel {
    padding: 7px 14px; border: 1px solid var(--rule); border-radius: var(--radius-input, 4px);
    background: var(--bg); color: var(--ink-2); font-family: var(--sans); font-size: 12.5px; cursor: pointer;
  }
  .btn-confirm {
    padding: 7px 14px; border: 1px solid rgba(196,58,58,0.5); border-radius: var(--radius-input, 4px);
    background: var(--color-error-light, #c43a3a); color: #fff;
    font-family: var(--sans); font-size: 12.5px; font-weight: 500; cursor: pointer;
  }
  .btn-confirm:disabled { opacity: 0.5; cursor: not-allowed; }
</style>
