<script lang="ts">
  import JunctionDot from './JunctionDot.svelte';
  import FeedAvatar from './FeedAvatar.svelte';
  import type { EntryListItem, Subscription } from '../lib/types';
  import { navigate } from '../lib/router';
  import { swipe } from '../lib/swipe';

  type Props = {
    entry: EntryListItem;
    feed: Subscription | undefined;
    isSelected?: boolean;
    onToggleRead?: () => void;
    onToggleSaved?: () => void;
  };
  let { entry, feed, isSelected = false, onToggleRead, onToggleSaved }: Props = $props();

  function ago(ts: number): string {
    const sec = Math.max(1, Math.floor(Date.now() / 1000) - ts);
    if (sec < 60) return `${sec}s ago`;
    if (sec < 3600) return `${Math.floor(sec / 60)}m ago`;
    if (sec < 86400) return `${Math.floor(sec / 3600)}h ago`;
    return `${Math.floor(sec / 86400)}d ago`;
  }
</script>

<button
  class="entry"
  class:is-read={entry.read}
  class:is-selected={isSelected}
  class:is-saved={entry.saved}
  aria-label="{entry.title}{entry.read ? ' (read)' : ''}"
  onclick={() => navigate(`/entry/${entry.id}`)}
  {@attach swipe({ onSwipeRight: onToggleRead, onSwipeLeft: onToggleSaved })}
>
  <span class="indicator junction" aria-hidden="true">
    <JunctionDot filled={!entry.read} />
  </span>
  {#if entry.saved}
    <span class="saved-mark" aria-hidden="true">SAVED</span>
  {/if}
  <div class="body">
    <div class="title">{entry.title}</div>
    <div class="meta">
      {#if feed}
        <FeedAvatar feedURL={feed.feed_url} size={9} radius={2} />
        <span class="src">{feed.title}</span>
        <span class="sep" aria-hidden="true">·</span>
      {/if}
      <span class="ago">{ago(entry.published_at)}</span>
    </div>
    <div class="summary">{entry.author ?? ''}</div>
  </div>
</button>

<style>
  .entry {
    display: flex; width: 100%; text-align: left;
    padding: 14px 24px 14px 40px;
    border-bottom: 1px solid var(--rule);
    position: relative;
    transition: background 120ms ease;
  }
  .entry:hover { background: var(--bg-soft); }
  .entry.is-selected { background: var(--accent-soft); }
  .indicator { position: absolute; left: 22px; top: 22px; }
  .saved-mark {
    position: absolute; right: 22px; top: 18px;
    font-family: var(--mono); font-size: 10px; letter-spacing: 0.04em;
    color: var(--accent);
  }
  .body { flex: 1; min-width: 0; }
  .title { font-family: var(--serif); font-size: 17px; line-height: 1.3; font-weight: 500; }
  .entry.is-read .title { font-weight: 400; color: var(--ink-3); }
  .meta {
    display: flex; align-items: center; gap: 6px;
    margin-top: 4px; font-family: var(--mono); font-size: 11px; color: var(--ink-3);
  }
  .src { font-weight: 500; color: var(--ink-2); font-family: var(--sans); }
  .sep { color: var(--ink-4); }
  .summary {
    font-family: var(--serif); font-size: 14px; line-height: 1.5; color: var(--ink-2);
    margin-top: 4px;
    display: -webkit-box; -webkit-line-clamp: 2; -webkit-box-orient: vertical; overflow: hidden;
  }
</style>
