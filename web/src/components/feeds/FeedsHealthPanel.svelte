<script lang="ts">
  import type { FeedRow } from '../../lib/feedsFilter';
  import { formatAgo } from '../../lib/url';

  type Props = {
    feed: FeedRow;
    onRetry: () => void;
    onEdit: () => void;
  };
  let { feed, onRetry, onEdit }: Props = $props();
</script>

<div class="ts-feed-health">
  <p class="ts-feed-health-eyebrow">Feed health</p>
  {#if feed.last_error}
    <p class="ts-feed-health-msg">{feed.last_error}</p>
  {/if}
  <div class="ts-feed-health-grid">
    <div class="ts-feed-health-cell">
      <span class="ts-feed-health-l">Consecutive errors</span>
      <span class="ts-feed-health-v">{feed.error_count}</span>
    </div>
    <div class="ts-feed-health-cell">
      <span class="ts-feed-health-l">Last try</span>
      <span class="ts-feed-health-v">{formatAgo(feed.lastPollAgo)} ago</span>
    </div>
    <div class="ts-feed-health-cell">
      <span class="ts-feed-health-l">Status</span>
      <span class="ts-feed-health-v">Error</span>
    </div>
  </div>
  <div class="ts-feed-health-actions">
    <button type="button" class="ts-feeds-util-btn" onclick={onRetry}>Retry now</button>
    <button type="button" class="ts-feeds-util-btn" onclick={onEdit}>Edit credentials</button>
  </div>
</div>

<style>
  .ts-feed-health {
    grid-column: 1 / -1;
    padding: 12px 16px;
    background: var(--bg-soft);
    border-top: 1px solid var(--rule);
    border-radius: 0 0 var(--radius-input, 4px) var(--radius-input, 4px);
  }
  .ts-feed-health-eyebrow {
    font-family: var(--mono); font-size: 10px; text-transform: uppercase;
    letter-spacing: 0.08em; color: var(--ink-3); margin: 0 0 4px;
  }
  .ts-feed-health-msg {
    font-family: var(--mono); font-size: 11.5px;
    color: var(--color-error-light, #c43a3a);
    margin: 0 0 12px;
  }
  :global(html.theme-dark) .ts-feed-health-msg { color: var(--color-error-dark, #ec7a7a); }
  .ts-feed-health-grid { display: flex; gap: 24px; margin-bottom: 12px; }
  .ts-feed-health-cell { display: flex; flex-direction: column; gap: 2px; }
  .ts-feed-health-l { font-family: var(--sans); font-size: 10.5px; color: var(--ink-3); text-transform: uppercase; letter-spacing: 0.06em; }
  .ts-feed-health-v { font-family: var(--mono); font-size: 12px; color: var(--ink-2); }
  .ts-feed-health-actions { display: flex; gap: 8px; }
  .ts-feeds-util-btn {
    display: inline-flex; align-items: center; gap: 5px;
    padding: 5px 8px;
    border: 1px solid var(--rule); background: var(--bg); color: var(--ink-2);
    font-family: var(--sans); font-size: 11.5px;
    cursor: pointer; border-radius: var(--radius-input, 4px);
  }
  .ts-feeds-util-btn:hover { color: var(--ink); background: var(--bg-soft); }
</style>
