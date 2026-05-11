<script lang="ts">
  import FeedAvatar from './FeedAvatar.svelte';
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

  function handleAction(ev: Event, fn?: () => void) {
    ev.stopPropagation();
    fn?.();
  }
</script>

<div
  class="row"
  class:is-read={entry.read}
  class:is-focused={isFocused}
  role="button"
  tabindex="0"
  onclick={() => onOpen?.()}
  onkeydown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); onOpen?.(); } }}
  onfocus={() => onFocus?.()}
  onmouseenter={() => onMouseEnter?.()}
>
  <span class="rail" aria-hidden="true">
    <span class="rail-mark">
      <svg width="10" height="10" viewBox="0 0 16 16" fill="currentColor" aria-hidden="true">
        <path d="M4 2.5h8v11l-4-3-4 3z" />
      </svg>
    </span>
  </span>

  <div class="body">
    <div class="eyebrow">
      <span>published {publishedLabel(entry.published_at)}</span>
      {#if entry.read}
        <span class="sep" aria-hidden="true">·</span>
        <span class="status-read">read</span>
      {/if}
    </div>

    <h3 class="title">{entry.title}</h3>

    <div class="byline">
      {#if feed}
        <FeedAvatar feedURL={feed.feed_url} size={11} radius={2} />
        <span class="source">{feed.title}</span>
      {/if}
      {#if entry.author && entry.author !== feed?.title}
        {#if feed}<span class="sep" aria-hidden="true">·</span>{/if}
        <span class="author">{entry.author}</span>
      {/if}
    </div>
  </div>

  <div class="actions" role="group" aria-label="Saved entry actions">
    <button
      type="button" class="action"
      onclick={(e) => handleAction(e, onOpen)}
      aria-label="Open entry"
    >
      <span>Open</span>
    </button>
    <button
      type="button" class="action"
      onclick={(e) => handleAction(e, onToggleRead)}
      aria-label={entry.read ? 'Mark unread' : 'Mark read'}
    >
      <span>{entry.read ? 'Mark unread' : 'Mark read'}</span>
    </button>
    <button
      type="button" class="action is-destructive"
      onclick={(e) => handleAction(e, onUnsave)}
      aria-label="Unsave"
    >
      <span>Unsave</span>
    </button>
  </div>
</div>

<style>
  .row {
    position: relative;
    display: grid;
    grid-template-columns: 20px 1fr;
    gap: 14px;
    padding: 18px 2px 18px 0;
    border: 0;
    border-bottom: 1px solid var(--rule);
    background: transparent;
    width: 100%;
    text-align: left;
    cursor: pointer;
    transition: background 100ms ease;
  }
  .row:hover { background: var(--bg-soft); }
  .row:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: 2px;
  }
  .row:last-child { border-bottom: 0; }

  .rail {
    position: relative;
    display: flex;
    justify-content: center;
    padding-top: 4px;
  }
  .rail-mark {
    width: 18px; height: 18px;
    display: inline-flex; align-items: center; justify-content: center;
    color: var(--accent);
  }
  .row.is-read .rail-mark { color: var(--ink-4); }

  .body { min-width: 0; }

  .eyebrow {
    display: flex; align-items: center; gap: 8px;
    font-family: var(--mono); font-size: 10px;
    letter-spacing: 0.06em; text-transform: uppercase;
    color: var(--ink-3);
    margin-bottom: 6px;
  }
  .status-read { color: var(--ink-3); }
  .row.is-read .status-read { color: var(--accent); }

  .title {
    font-family: var(--serif); font-size: 20px; line-height: 1.25;
    font-weight: 500; color: var(--ink);
    margin: 0 0 6px;
    text-wrap: pretty;
    letter-spacing: -0.005em;
  }
  .row.is-read .title { color: var(--ink-2); font-weight: 400; }

  .byline {
    display: flex; align-items: center; gap: 8px;
    font-family: var(--sans); font-size: 12.5px;
    color: var(--ink-2);
    margin-bottom: 6px;
    flex-wrap: wrap;
  }
  .source { color: var(--ink); font-weight: 500; }
  .row.is-read .source { color: var(--ink-2); font-weight: 400; }
  .author { color: var(--ink-2); font-style: italic; }

  .sep::before {
    content: ""; display: inline-block;
    width: 3px; height: 3px; border-radius: 50%;
    background: var(--ink-4);
    vertical-align: middle;
  }

  .actions {
    grid-column: 1 / -1;
    display: flex;
    gap: 4px;
    margin-top: 4px;
    margin-left: 34px;
    max-height: 0;
    opacity: 0;
    overflow: hidden;
    transition: max-height 160ms ease, opacity 120ms ease, margin-top 160ms ease;
  }
  .row:hover .actions,
  .row:focus-within .actions {
    max-height: 50px;
    opacity: 1;
    margin-top: 8px;
  }
  .action {
    display: inline-flex; align-items: center; gap: 7px;
    padding: 6px 10px;
    font-family: var(--sans); font-size: 12px; font-weight: 500;
    color: var(--ink-2);
    background: transparent;
    border: 0;
    border-radius: 3px;
    cursor: pointer;
    transition: background 100ms ease, color 100ms ease;
  }
  .action:hover { color: var(--ink); background: var(--bg); }
  .action.is-destructive:hover { color: var(--accent); }
</style>
