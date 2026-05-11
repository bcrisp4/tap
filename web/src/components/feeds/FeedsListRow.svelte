<script lang="ts">
  import FeedAvatar from '../FeedAvatar.svelte';
  import FeedsHealthPanel from './FeedsHealthPanel.svelte';
  import type { FeedRow } from '../../lib/feedsFilter';
  import { displayUrl, originOf, formatAgo } from '../../lib/url';

  type Props = {
    feed: FeedRow;
    categoryName: string | null;
    isSelected: boolean;
    anySelected: boolean;
    isExpanded: boolean;
    isRefreshing: boolean;
    onToggleSelect: () => void;
    onToggleExpand: () => void;
    onRefresh: () => void;
    onChangeCategory: (anchor: HTMLElement) => void;
    onEdit: () => void;
    onDelete: () => void;
  };

  let {
    feed, categoryName, isSelected, anySelected, isExpanded, isRefreshing,
    onToggleSelect, onToggleExpand, onRefresh, onChangeCategory, onEdit, onDelete,
  }: Props = $props();

  let catChipEl = $state<HTMLElement | null>(null);
</script>

<div
  class="ts-feed-row"
  class:is-selected={isSelected}
  class:any-selected={anySelected}
  class:has-error={feed.error_count > 0}
  class:is-busy={isRefreshing}
>
  <button
    class="ts-feed-check"
    class:is-checked={isSelected}
    type="button"
    onclick={onToggleSelect}
    aria-label={isSelected ? 'Deselect' : 'Select'}
  >
    {#if isSelected}
      <svg width="10" height="10" viewBox="0 0 10 10" fill="none" aria-hidden="true">
        <polyline points="1,5 4,8 9,2" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/>
      </svg>
    {/if}
  </button>

  <span class="ts-feed-avatar-wrap">
    <FeedAvatar feedURL={feed.feed_url} size={18} />
  </span>

  <div class="ts-feed-body">
    <div class="ts-feed-line1">
      <h3 class="ts-feed-name">{feed.title}</h3>
      <button
        class="ts-feed-cat"
        class:is-uncat={!feed.category_id}
        type="button"
        bind:this={catChipEl}
        onclick={() => onChangeCategory(catChipEl!)}
      >
        {categoryName ?? 'uncategorised'}
      </button>
      {#if feed.error_count > 0}
        <span class="ts-feed-err-chip" title={feed.last_error ?? ''}>
          <svg width="10" height="10" viewBox="0 0 12 12" fill="none" aria-hidden="true">
            <path d="M6 1L11 10H1L6 1z" stroke="currentColor" stroke-width="1.2" stroke-linejoin="round"/>
            <line x1="6" y1="5" x2="6" y2="7.5" stroke="currentColor" stroke-width="1.2" stroke-linecap="round"/>
            <circle cx="6" cy="9" r="0.6" fill="currentColor"/>
          </svg>
          {feed.error_count} {feed.error_count === 1 ? 'error' : 'errors'}
        </span>
      {/if}
    </div>
    <div class="ts-feed-line2">
      <a
        class="ts-feed-url"
        href={feed.site_url || originOf(feed.feed_url)}
        target="_blank"
        rel="noopener noreferrer"
      >
        {displayUrl(feed.feed_url)}
        <svg width="10" height="10" viewBox="0 0 12 12" fill="none" aria-hidden="true" class="ico">
          <path d="M5 2H2v8h8V7M7 2h3m0 0v3M10 2L5.5 6.5" stroke="currentColor" stroke-width="1.2" stroke-linecap="round" stroke-linejoin="round"/>
        </svg>
      </a>
      <span class="dot" aria-hidden="true"></span>
      {#if feed.error_count > 0}
        <span class="err">last poll <b>{formatAgo(feed.lastPollAgo)}</b> ago</span>
        <span class="dot" aria-hidden="true"></span>
        <button class="ts-feeds-util-btn" type="button" onclick={onToggleExpand}>
          {isExpanded ? 'Hide details' : 'Why?'}
        </button>
      {:else}
        <span>polled <b>{formatAgo(feed.lastPollAgo)}</b> ago</span>
        <span class="dot" aria-hidden="true"></span>
        <span class="unread">unread <b>{feed.unread}</b></span>
      {/if}
    </div>
  </div>

  <div class="ts-feed-actions">
    <span class="ts-feed-act-unread" class:is-zero={feed.unread === 0}>
      <b>{feed.unread}</b><span>unread</span>
    </span>
    <button
      class="ts-feed-act"
      class:is-spinning={isRefreshing}
      data-action="refresh"
      type="button"
      onclick={onRefresh}
      aria-label="Refresh now"
    >
      <svg width="13" height="13" viewBox="0 0 16 16" fill="none" aria-hidden="true">
        <path d="M13.5 2.5A7 7 0 1 0 15 8" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/>
        <polyline points="11,0 14,3 11,6" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/>
      </svg>
    </button>
    <button
      class="ts-feed-act"
      data-action="delete"
      type="button"
      onclick={onDelete}
      aria-label="Delete feed"
    >
      <svg width="13" height="13" viewBox="0 0 16 16" fill="none" aria-hidden="true">
        <polyline points="2,4 14,4" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"/>
        <path d="M6 4V2h4v2M5 4v9h6V4" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round"/>
      </svg>
    </button>
    <button
      class="ts-feed-act"
      data-action="edit"
      type="button"
      onclick={onEdit}
      aria-label="Edit feed"
    >
      <svg width="13" height="13" viewBox="0 0 16 16" fill="none" aria-hidden="true">
        <circle cx="8" cy="8" r="1.2" fill="currentColor"/>
        <circle cx="3" cy="8" r="1.2" fill="currentColor"/>
        <circle cx="13" cy="8" r="1.2" fill="currentColor"/>
      </svg>
    </button>
  </div>

  {#if isExpanded && feed.error_count > 0}
    <FeedsHealthPanel {feed} onRetry={onRefresh} {onEdit} />
  {/if}
</div>

<style>
  .ts-feed-row {
    display: grid;
    grid-template-columns: 24px 26px 1fr auto;
    grid-template-rows: auto auto;
    align-items: center;
    gap: 0 10px;
    padding: 10px 12px;
    border-bottom: 1px solid var(--rule);
  }
  .ts-feed-row:hover { background: var(--bg-soft); }
  .ts-feed-row.is-selected { background: color-mix(in srgb, var(--accent) 8%, var(--bg)); }
  .ts-feed-row.has-error .ts-feed-name { color: var(--color-error-light, #c43a3a); }
  :global(html.theme-dark) .ts-feed-row.has-error .ts-feed-name { color: var(--color-error-dark, #ec7a7a); }

  .ts-feed-check {
    display: flex; align-items: center; justify-content: center;
    width: 18px; height: 18px;
    border: 1px solid var(--rule);
    border-radius: 3px;
    background: var(--bg);
    cursor: pointer;
    color: var(--ink-2);
  }
  .ts-feed-check.is-checked { background: var(--ink); color: var(--bg); border-color: var(--ink); }
  .ts-feed-avatar-wrap { display: flex; align-items: center; }

  .ts-feed-body { min-width: 0; }
  .ts-feed-line1 { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; margin-bottom: 2px; }
  .ts-feed-name { font-family: var(--sans); font-size: 13.5px; font-weight: 500; color: var(--ink); margin: 0; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .ts-feed-cat {
    font-family: var(--sans); font-size: 11px; color: var(--ink-3);
    border: 1px solid var(--rule); border-radius: var(--radius-pill, 100px);
    padding: 1px 8px; background: var(--bg); cursor: pointer;
  }
  .ts-feed-cat:hover { color: var(--ink); border-color: var(--ink-4); }
  .ts-feed-cat.is-uncat { color: var(--ink-4); border-style: dashed; }
  .ts-feed-err-chip {
    display: inline-flex; align-items: center; gap: 4px;
    font-family: var(--mono); font-size: 10px; text-transform: uppercase;
    color: var(--color-error-light, #c43a3a); letter-spacing: 0.06em;
  }
  :global(html.theme-dark) .ts-feed-err-chip { color: var(--color-error-dark, #ec7a7a); }

  .ts-feed-line2 { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
  .ts-feed-url { font-family: var(--mono); font-size: 11px; color: var(--ink-3); text-decoration: none; display: inline-flex; align-items: center; gap: 3px; }
  .ts-feed-url:hover { color: var(--ink-2); }
  .dot { width: 3px; height: 3px; border-radius: 50%; background: var(--ink-4); }
  .err { font-family: var(--sans); font-size: 11.5px; color: var(--color-error-light, #c43a3a); }
  :global(html.theme-dark) .err { color: var(--color-error-dark, #ec7a7a); }
  .ts-feeds-util-btn {
    padding: 2px 6px; border: none; background: transparent;
    font-family: var(--sans); font-size: 11.5px; color: var(--ink-3); cursor: pointer;
    border-radius: 3px;
  }
  .ts-feeds-util-btn:hover { color: var(--ink); background: var(--bg-soft); }

  .ts-feed-actions { display: flex; align-items: center; gap: 4px; }
  .ts-feed-act-unread { display: flex; flex-direction: column; align-items: center; gap: 1px; min-width: 36px; }
  .ts-feed-act-unread b { font-family: var(--mono); font-size: 13px; color: var(--ink-2); line-height: 1; }
  .ts-feed-act-unread span { font-family: var(--sans); font-size: 9px; text-transform: uppercase; color: var(--ink-4); letter-spacing: 0.05em; }
  .ts-feed-act-unread.is-zero b { color: var(--ink-4); }
  .ts-feed-act {
    display: flex; align-items: center; justify-content: center;
    width: 28px; height: 28px;
    border: none; background: transparent; color: var(--ink-3);
    cursor: pointer; border-radius: var(--radius-input, 4px);
  }
  .ts-feed-act:hover { color: var(--ink); background: var(--bg-soft); }
  @keyframes ts-spin { to { transform: rotate(360deg); } }
  .ts-feed-act.is-spinning svg { animation: ts-spin 0.8s linear infinite; }

  .ts-feed-row :global(.ts-feed-health) { grid-column: 1 / -1; }
</style>
