<script lang="ts">
  import FeedAvatar from './FeedAvatar.svelte';
  import type { EntryListItem, Subscription } from '../lib/types';
  import { navigate } from '../lib/router';
  import { swipe } from '../lib/swipe';

  type Density = 'compact' | 'comfortable' | 'cosy';
  type Props = {
    entry: EntryListItem;
    feed: Subscription | undefined;
    isSelected?: boolean;
    density?: Density;
    onToggleRead?: () => void;
    onToggleSaved?: () => void;
  };
  let {
    entry, feed, isSelected = false, density = 'comfortable',
    onToggleRead, onToggleSaved,
  }: Props = $props();

  function ago(ts: number): string {
    const sec = Math.max(1, Math.floor(Date.now() / 1000) - ts);
    if (sec < 60) return `${sec}s`;
    if (sec < 3600) return `${Math.floor(sec / 60)}m`;
    if (sec < 86400) return `${Math.floor(sec / 3600)}h`;
    return `${Math.floor(sec / 86400)}d`;
  }
</script>

<button
  class="entry density-{density}"
  class:is-read={entry.read}
  class:is-selected={isSelected}
  class:is-saved={entry.saved}
  aria-label="{entry.title}{entry.read ? ' (read)' : ''}"
  onclick={() => navigate(`/entry/${entry.id}`)}
  {@attach swipe({ onSwipeRight: onToggleRead, onSwipeLeft: onToggleSaved })}
>
  <span class="junction" aria-hidden="true"></span>
  {#if entry.saved}<span class="saved-mark" aria-label="Saved">SAVED</span>{/if}
  <h3 class="title">{entry.title}</h3>
  <div class="meta">
    {#if feed}
      <FeedAvatar feedURL={feed.feed_url} size={10} radius={2} />
      <span class="source">{feed.title}</span>
      <span class="sep" aria-hidden="true"></span>
    {/if}
    <span class="ago">{ago(entry.published_at)} ago</span>
  </div>
  {#if entry.author && density !== 'compact'}
    <p class="summary">{entry.author}</p>
  {/if}
</button>

<style>
  .entry {
    position: relative;
    padding: 16px 24px 16px 40px;
    border-bottom: 1px solid var(--rule);
    cursor: pointer;
    transition: background var(--dur-fast, 100ms) ease;
    width: 100%;
    text-align: left;
    background: transparent;
    border-left: 0; border-right: 0; border-top: 0;
    display: block;
  }
  .entry:hover { background: var(--bg-soft); }
  .entry.is-selected { background: var(--accent-soft); }
  .entry.is-read .title { color: var(--ink-3); font-weight: 400; }
  .entry.is-read .meta { color: var(--ink-3); }
  /* Brand spec §4.4: comfortable is the default (16px pad, summary 2 lines). */
  .entry.density-compact { padding-top: 10px; padding-bottom: 10px; }
  .entry.density-compact .summary { display: none; }
  .entry.density-compact .junction { top: 17px; }
  .entry.density-cosy { padding-top: 10px; padding-bottom: 10px; }
  .entry.density-cosy .junction { top: 17px; }
  .entry.density-cosy .summary { -webkit-line-clamp: 1; }

  .junction {
    position: absolute;
    left: 22px; top: 22px;
    width: 6px; height: 6px;
    border-radius: 50%;
    background: var(--accent);
    transition: transform 200ms ease, background 200ms ease;
  }
  .entry.is-read .junction { background: transparent; border: 1px solid var(--ink-4); }

  .saved-mark {
    position: absolute;
    right: 22px; top: 18px;
    color: var(--accent);
    font-family: var(--mono);
    font-size: 10px;
    letter-spacing: 0.04em;
  }

  .title {
    font-family: var(--serif);
    font-size: var(--fs-entry-title, 19px);
    line-height: var(--lh-entry-title, 1.3);
    font-weight: 500;
    color: var(--ink);
    margin: 0 0 5px;
    text-wrap: pretty;
    letter-spacing: var(--tr-entry-title, -0.005em);
  }

  .meta {
    font-family: var(--sans);
    font-size: 12px;
    color: var(--ink-2);
    display: flex;
    align-items: center;
    gap: 8px;
    margin-top: 4px;
    flex-wrap: wrap;
  }
  .meta .source { color: var(--ink); font-weight: 500; }
  .meta .sep {
    display: inline-block;
    width: 3px; height: 3px;
    background: var(--ink-4);
    border-radius: 50%;
  }
  .meta .ago { font-family: var(--mono); font-size: 10.5px; color: var(--ink-3); }

  .summary {
    font-family: var(--serif);
    font-size: var(--fs-entry-summary, 14.5px);
    line-height: var(--lh-entry-summary, 1.55);
    color: var(--ink-2);
    margin: 6px 0 0;
    display: -webkit-box;
    -webkit-line-clamp: 2;
    -webkit-box-orient: vertical;
    overflow: hidden;
    text-wrap: pretty;
  }

  :global(.is-mobile) .entry { padding-left: 36px; padding-right: 18px; }
  :global(.is-mobile) .entry .junction { left: 18px; top: 22px; }
  :global(.is-mobile) .entry .title { font-size: var(--fs-entry-title-mobile, 16px); }
</style>
