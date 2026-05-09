<script lang="ts">
  import JunctionDot from './JunctionDot.svelte';
  import FeedAvatar from './FeedAvatar.svelte';
  import type { EntryListItem, Subscription } from '../lib/types';
  import { navigate } from '../lib/router';

  type Props = {
    entry: EntryListItem;
    feed: Subscription | undefined;
  };
  let { entry, feed }: Props = $props();

  function ago(ts: number): string {
    const sec = Math.max(1, Math.floor(Date.now() / 1000) - ts);
    if (sec < 60) return `${sec}s ago`;
    if (sec < 3600) return `${Math.floor(sec / 60)}m ago`;
    if (sec < 86400) return `${Math.floor(sec / 3600)}h ago`;
    return `${Math.floor(sec / 86400)}d ago`;
  }
</script>

<button class="entry" onclick={() => navigate(`/entry/${entry.id}`)}>
  <span class="indicator">
    <JunctionDot filled={!entry.read} />
  </span>
  <div class="body">
    <div class="title" class:read={entry.read}>{entry.title}</div>
    <div class="meta">
      {#if feed}
        <FeedAvatar feedURL={feed.feed_url} size={9} radius={2} />
        <span class="src">{feed.title}</span>
        <span class="sep">·</span>
      {/if}
      <span class="ago">{ago(entry.published_at)}</span>
    </div>
  </div>
</button>

<style>
  .entry {
    display: flex;
    width: 100%;
    text-align: left;
    padding: 14px 24px 14px 40px;
    border-bottom: 1px solid var(--rule);
    position: relative;
  }
  .entry:hover { background: var(--bg-soft); }
  .indicator {
    position: absolute;
    left: 22px;
    top: 22px;
  }
  .body { flex: 1; min-width: 0; }
  .title {
    font-family: var(--serif);
    font-size: 17px;
    line-height: 1.3;
    font-weight: 500;
  }
  .title.read {
    font-weight: 400;
    color: var(--ink-3);
  }
  .meta {
    display: flex;
    align-items: center;
    gap: 6px;
    margin-top: 4px;
    font-family: var(--mono);
    font-size: 11px;
    color: var(--ink-3);
  }
  .src { font-weight: 500; color: var(--ink-2); font-family: var(--sans); }
  .sep { color: var(--ink-4); }
</style>
