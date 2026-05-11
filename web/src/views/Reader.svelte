<script lang="ts">
  import { getContext, onMount, onDestroy } from 'svelte';
  import { api } from '../lib/api';
  import { navigate } from '../lib/router';
  import { entries } from '../lib/store';
  import { swipe } from '../lib/swipe';
  import type { EntryDetail } from '../lib/types';
  import FeedAvatar from '../components/FeedAvatar.svelte';
  import Button from '../components/Button.svelte';

  type Props = { id: number };
  let { id }: Props = $props();

  let entry = $state<EntryDetail | null>(null);
  let error = $state<string | null>(null);

  const dispatch = getContext<{
    onToggleRead: () => void;
    onToggleSaved: () => void;
    onViewOriginal: () => void;
  }>('keyDispatch');

  $effect(() => {
    const targetId = id;
    entry = null; error = null;
    let cancelled = false;
    (async () => {
      try {
        const fetched = await api.getEntry(targetId);
        if (cancelled) return;
        entry = fetched;
        if (fetched && !fetched.read) {
          try {
            await entries.toggleRead(targetId, true);
            if (cancelled) return;
            entry = { ...fetched, read: true };
          } catch { /* swallow — reader still shows content */ }
        }
      } catch (e) {
        if (cancelled) return;
        error = (e as Error).message;
      }
    })();
    return () => { cancelled = true; };
  });

  async function toggleRead() {
    if (!entry) return;
    const want = !entry.read;
    entry = { ...entry, read: want };
    try {
      await entries.toggleRead(entry.id, want);
      error = null;
    } catch (e) {
      entry = { ...entry, read: !want };
      error = (e as Error).message;
    }
  }

  async function toggleSaved() {
    if (!entry) return;
    const want = !entry.saved;
    entry = { ...entry, saved: want };
    try {
      await entries.toggleSaved(entry.id, want);
      error = null;
    } catch (e) {
      entry = { ...entry, saved: !want };
      error = (e as Error).message;
    }
  }

  function viewOriginal() {
    if (entry) window.open(entry.url, '_blank', 'noopener');
  }

  function navigateRelative(delta: -1 | 1) {
    const items = $entries.items;
    const idx = items.findIndex(e => e.id === id);
    if (idx === -1) return;
    const next = items[idx + delta];
    if (next) navigate(`/entry/${next.id}`);
  }

  function host(url: string): string {
    try { return new URL(url).host; } catch { return ''; }
  }

  if (dispatch) {
    onMount(() => {
      dispatch.onToggleRead = toggleRead;
      dispatch.onToggleSaved = toggleSaved;
      dispatch.onViewOriginal = viewOriginal;
    });
    onDestroy(() => {
      dispatch.onToggleRead = () => {};
      dispatch.onToggleSaved = () => {};
      dispatch.onViewOriginal = () => {};
    });
  }
</script>

<div class="reader-pane">
  <header class="reader-header">
    <Button variant="quiet" onclick={() => navigate('/')} title="Back to unread entries">
      ‹ Back
    </Button>
    {#if entry}
      <div class="reader-actions">
        <Button variant="quiet" onclick={toggleRead}>
          {entry.read ? 'Mark unread' : 'Mark read'}
        </Button>
        <Button variant="quiet" onclick={toggleSaved}>
          {entry.saved ? 'Saved' : 'Save'}
        </Button>
        <a class="view-link" href={entry.url} target="_blank" rel="noopener">View original</a>
      </div>
    {/if}
  </header>

  <article
    class="reader-body"
    {@attach swipe({
      onSwipeRight: () => navigateRelative(-1),
      onSwipeLeft:  () => navigateRelative(1),
    })}
  >
    {#if error}
      <p class="err">{error}</p>
    {:else if !entry}
      <p class="loading">Loading…</p>
    {:else}
      <div class="reader-source">
        <FeedAvatar feedURL={host(entry.url)} size={10} radius={2} />
        <span class="src-host">{host(entry.url)}</span>
      </div>
      <h1 class="reader-title">{entry.title}</h1>
      <div class="reader-byline">
        {#if entry.author}<span>{entry.author}</span><span aria-hidden="true"> · </span>{/if}
        <span>{new Date(entry.published_at * 1000).toLocaleDateString()}</span>
      </div>
      <div class="reader-rule">
        <span class="reader-rule-line"></span>
        <span class="reader-rule-dot" aria-hidden="true"></span>
        <span class="reader-rule-line"></span>
      </div>
      <div class="content">{@html entry.content}</div>
      <div class="reader-end">
        <span class="reader-end-line"></span>
        <span class="reader-end-dot" aria-hidden="true"></span>
        <span class="reader-end-line"></span>
      </div>
    {/if}
  </article>
</div>

<style>
  .reader-pane { display: flex; flex-direction: column; min-height: 0; }
  .reader-header {
    display: flex; align-items: center; justify-content: space-between;
    padding: 8px 0;
    border-bottom: 1px solid var(--rule);
    margin-bottom: 16px;
  }
  .reader-actions { display: flex; gap: 4px; }
  .view-link {
    display: inline-flex; align-items: center;
    padding: 6px 8px;
    font-family: var(--sans); font-size: 12.5px;
    color: var(--ink-2);
    border-radius: 4px;
    text-decoration: none;
  }
  .view-link:hover { color: var(--ink); background: var(--bg-soft); }
  .err { color: #b14; font-family: var(--mono); font-size: 12px; }
  .loading { color: var(--ink-3); font-family: var(--mono); font-size: 11px; }
  .reader-source { display: flex; align-items: center; gap: 8px; margin-bottom: 18px; }
  .src-host { font-family: var(--mono); font-size: 11px; color: var(--ink-3); }
  .reader-title {
    font-family: var(--serif); font-size: 38px; line-height: 1.15;
    font-weight: 600; color: var(--ink); letter-spacing: -0.015em;
    margin: 0 0 16px; text-wrap: balance;
  }
  .reader-byline {
    font-size: 11px; letter-spacing: 0.04em; color: var(--ink-3);
    display: flex; gap: 8px; flex-wrap: wrap;
    margin-bottom: 28px; text-transform: uppercase;
  }
  .reader-rule {
    display: flex; align-items: center; gap: 10px;
    margin: 0 0 32px;
  }
  .reader-rule-line { flex: 1; height: 1px; background: var(--rule); }
  .reader-rule-dot { width: 6px; height: 6px; border-radius: 50%; background: var(--accent); flex-shrink: 0; }
  .reader-end {
    display: flex; align-items: center; gap: 10px;
    margin: 48px 0 18px;
  }
  .reader-end-line { flex: 1; height: 1px; background: var(--rule); }
  .reader-end-dot { width: 6px; height: 6px; border-radius: 50%; background: var(--ink-4); flex-shrink: 0; }
  .content :global(p) {
    font-family: var(--serif); font-size: 17px; line-height: 1.7;
    color: var(--ink); margin: 0 0 22px; text-wrap: pretty;
  }
  .content :global(h2) {
    font-family: var(--serif); font-size: 22px; line-height: 1.25;
    font-weight: 600; letter-spacing: -0.01em; margin: 40px 0 14px;
  }
  .content :global(pre), .content :global(code) {
    font-family: var(--mono); font-size: 13px; line-height: 1.55;
    color: var(--ink-2); background: var(--bg-soft);
    padding: 14px 16px; border-left: 2px solid var(--accent); overflow-x: auto;
  }
</style>
