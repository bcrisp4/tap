<script lang="ts">
  import { getContext, onMount, onDestroy } from 'svelte';
  import { api } from '../lib/api';
  import { navigate } from '../lib/router';
  import { entries } from '../lib/store';
  import { swipe } from '../lib/swipe';
  import type { EntryDetail } from '../lib/types';
  import FeedAvatar from '../components/FeedAvatar.svelte';
  import JunctionDot from '../components/JunctionDot.svelte';
  import Sidebar from '../components/Sidebar.svelte';

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
    } catch {
      entry = { ...entry, read: !want };
    }
  }

  async function toggleSaved() {
    if (!entry) return;
    const want = !entry.saved;
    entry = { ...entry, saved: want };
    try {
      await entries.toggleSaved(entry.id, want);
    } catch {
      entry = { ...entry, saved: !want };
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

<div class="layout">
  <Sidebar />
  <div class="reader-pane">
    <header class="reader-header">
      <button class="reader-back" onclick={() => navigate('/')} aria-label="Back to unread entries">
        ‹ <span>UNREAD</span>
      </button>
      {#if entry}
        <div class="reader-actions">
          <button class="reader-action" onclick={toggleRead}>
            {entry.read ? 'MARK UNREAD' : 'MARK READ'}
          </button>
          <button class="reader-action" class:is-saved={entry.saved} onclick={toggleSaved}>
            {entry.saved ? 'SAVED' : 'SAVE'}
          </button>
          <a class="reader-action" href={entry.url} target="_blank" rel="noopener">VIEW ORIGINAL</a>
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
          <JunctionDot color="var(--accent)" />
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
</div>

<style>
  .layout { display: flex; height: 100vh; }
  .err { color: #b14; font-family: var(--mono); font-size: 12px; }
  .loading { color: var(--ink-3); font-family: var(--mono); font-size: 11px; }
  .src-host { font-family: var(--mono); font-size: 11px; color: var(--ink-3); }
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
