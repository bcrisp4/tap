<script lang="ts">
  import Dialog from './Dialog.svelte';

  type Props = {
    categoryName: string;
    unread: number;
    feedCount: number;
    onCancel: () => void;
    onConfirm: () => void;
  };
  const { categoryName, unread, feedCount, onCancel, onConfirm }: Props = $props();
</script>

<Dialog
  open={true}
  title={`Mark ${unread} ${unread === 1 ? 'entry' : 'entries'} as read?`}
  onClose={onCancel}
>
  <p class="ts-dialog-p">
    Everything currently unread in <b>{categoryName}</b> will be marked as read.
    Anything you've saved stays saved &mdash; only the unread dot disappears.
  </p>
  <div class="ts-dialog-stats">
    <span><b>{unread}</b> unread</span>
    <span class="dot" aria-hidden="true"></span>
    <span><b>{feedCount}</b> {feedCount === 1 ? 'feed' : 'feeds'}</span>
    <span class="dot" aria-hidden="true"></span>
    <span>across {categoryName}</span>
  </div>
  {#snippet foot()}
    <div class="ts-dialog-foot-l">esc to cancel</div>
    <button class="ts-btn" onclick={onCancel}>Cancel</button>
    <button class="ts-btn is-primary" onclick={onConfirm}>Mark all read</button>
  {/snippet}
</Dialog>

<style>
  .ts-dialog-p {
    font-family: var(--serif); font-size: 15px; line-height: 1.55;
    color: var(--ink-2); margin: 0;
  }
  .ts-dialog-stats {
    display: flex; align-items: center; gap: 6px;
    margin-top: 14px; padding-top: 12px; border-top: 1px solid var(--rule);
    font-family: var(--mono); font-size: 11px; color: var(--ink-3);
    letter-spacing: 0.02em;
  }
  .ts-dialog-stats b {
    color: var(--ink); font-weight: 500;
    font-feature-settings: "tnum";
  }
  .ts-dialog-stats .dot {
    display: inline-block; width: 3px; height: 3px; border-radius: 50%;
    background: var(--ink-4); margin: 0 4px;
  }
  .ts-dialog-foot-l {
    font-family: var(--mono); font-size: 10.5px; color: var(--ink-3);
    letter-spacing: 0.06em; margin-right: auto;
  }
</style>
