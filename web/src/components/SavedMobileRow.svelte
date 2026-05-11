<script lang="ts">
  import FeedAvatar from './FeedAvatar.svelte';
  import { swipe } from '../lib/swipe';
  import type { EntryListItem, Subscription } from '../lib/types';

  type Props = {
    entry: EntryListItem;
    feed: Subscription | undefined;
    isFocused?: boolean;
    onFocus?: () => void;
    onMouseEnter?: () => void;
    onOpen?: () => void;
    onToggleRead?: () => void;
    onUnsave?: () => void;
  };
  let {
    entry, feed,
    isFocused = false,
    onFocus, onMouseEnter,
    onOpen, onToggleRead, onUnsave,
  }: Props = $props();

  function publishedLabel(ts: number): string {
    return new Date(ts * 1000).toLocaleDateString(undefined, {
      month: 'short', day: 'numeric', year: 'numeric',
    });
  }
</script>

<div
  class="row"
  class:is-read={entry.read}
  class:is-focused={isFocused}
  {@attach swipe({
    onSwipeLeft: () => onUnsave?.(),
    onSwipeRight: () => onToggleRead?.(),
  })}
>
  <div class="rev rev-left" aria-hidden="true">
    <span>{entry.read ? 'Mark unread' : 'Mark read'}</span>
  </div>
  <div class="rev rev-right" aria-hidden="true">
    <span>Unsave</span>
  </div>

  <button type="button" class="card" onclick={() => onOpen?.()} onfocus={() => onFocus?.()} onmouseenter={() => onMouseEnter?.()}>
    <div class="eyebrow">
      <span>saved</span>
      {#if entry.read}
        <span class="sep" aria-hidden="true">·</span>
        <span class="status-read">read</span>
      {/if}
    </div>
    <h3 class="title">{entry.title}</h3>
    <div class="byline">
      {#if feed}
        <FeedAvatar feedURL={feed.feed_url} size={10} radius={2} />
        <span class="source">{feed.title}</span>
      {/if}
      {#if entry.author && entry.author !== feed?.title}
        {#if feed}<span class="sep" aria-hidden="true">·</span>{/if}
        <span>{entry.author}</span>
      {/if}
    </div>
    <div class="foot">
      <span>published {publishedLabel(entry.published_at)}</span>
    </div>
  </button>
</div>

<style>
  .row {
    position: relative;
    overflow: hidden;
    border-bottom: 1px solid var(--rule);
    background: var(--bg);
  }
  .rev {
    position: absolute;
    top: 0; bottom: 0;
    display: flex; align-items: center; gap: 8px;
    padding: 0 22px;
    font-family: var(--sans); font-size: 13px; font-weight: 500;
    letter-spacing: 0.01em;
  }
  .rev-left {
    left: 0;
    background: var(--bg-soft);
    color: var(--ink-2);
    border-right: 1px solid var(--rule);
  }
  .rev-right {
    right: 0;
    background: var(--accent);
    color: #fff;
    justify-content: flex-end;
  }
  :global(.theme-dark) .rev-right { color: #0d0d0e; }

  .card {
    position: relative;
    background: var(--bg);
    padding: 14px 18px 16px;
    width: 100%;
    display: block;
    border: 0;
    text-align: left;
    cursor: pointer;
    z-index: 1;
    transition: transform 240ms cubic-bezier(.2,.7,.2,1);
  }
  .card:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: -2px;
  }

  .eyebrow {
    display: flex; align-items: center; gap: 7px;
    font-family: var(--mono); font-size: 10px;
    letter-spacing: 0.06em; text-transform: uppercase;
    color: var(--ink-3);
    margin-bottom: 6px;
  }
  .status-read { color: var(--ink-3); }
  .row.is-read .status-read { color: var(--accent); }

  .title {
    font-family: var(--serif); font-size: 17px; line-height: 1.3;
    font-weight: 500; color: var(--ink);
    margin: 0 0 6px;
    letter-spacing: -0.005em;
    text-wrap: pretty;
  }
  .row.is-read .title { color: var(--ink-2); font-weight: 400; }

  .byline {
    display: flex; align-items: center; gap: 7px;
    font-family: var(--sans); font-size: 12px; color: var(--ink-2);
    margin-bottom: 6px;
    flex-wrap: wrap;
  }
  .source { color: var(--ink); font-weight: 500; }
  .row.is-read .source { color: var(--ink-2); font-weight: 400; }

  .foot {
    display: flex; align-items: center; gap: 8px;
    font-family: var(--mono); font-size: 10px;
    letter-spacing: 0.04em; text-transform: uppercase;
    color: var(--ink-3);
  }

  .sep::before {
    content: ""; display: inline-block;
    width: 3px; height: 3px; border-radius: 50%;
    background: var(--ink-4);
    vertical-align: middle;
  }
</style>
