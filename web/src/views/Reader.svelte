<script lang="ts">
  import { getContext, onMount, onDestroy } from 'svelte';
  import { api } from '../lib/api';
  import { navigate } from '../lib/router';
  import { entries } from '../lib/store';
  import type { EntryDetail } from '../lib/types';
  import FeedAvatar from '../components/FeedAvatar.svelte';
  import { measure, font, markOnScroll } from '../lib/preferences.svelte';
  import { loadScroll, saveScroll } from '../lib/readerScroll';
  import { createMarkOnScroll } from '../lib/markOnScroll';
  import { isMobile } from '../lib/breakpoints.svelte';

  type Props = { id: number };
  let { id }: Props = $props();

  let entry = $state<EntryDetail | null>(null);
  let error = $state<string | null>(null);
  let scrollEl = $state<HTMLElement | null>(null);
  let saveTimer: ReturnType<typeof setTimeout> | null = null;

  // Snapshot pref once at component setup so mid-session pref changes don't
  // race a half-fired observer.
  const useMarkOnScroll = markOnScroll.value;

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
        if (!useMarkOnScroll && fetched && !fetched.read) {
          try {
            await entries.toggleRead(targetId, true);
            if (cancelled) return;
            entry = { ...fetched, read: true };
          } catch { /* swallow */ }
        }
        requestAnimationFrame(() => {
          if (cancelled || !scrollEl) return;
          const saved = loadScroll(targetId);
          if (saved > 0) scrollEl.scrollTop = saved;
        });
      } catch (e) {
        if (cancelled) return;
        error = (e as Error).message;
      }
    })();
    return () => { cancelled = true; };
  });

  function onScroll() {
    if (!scrollEl || !entry) return;
    if (saveTimer !== null) clearTimeout(saveTimer);
    const idCopy = entry.id;
    const top = scrollEl.scrollTop;
    saveTimer = setTimeout(() => saveScroll(idCopy, top), 300);
  }

  async function toggleRead() {
    if (!entry) return;
    const want = !entry.read;
    entry = { ...entry, read: want };
    try { await entries.toggleRead(entry.id, want); error = null; }
    catch (e) { entry = { ...entry, read: !want }; error = (e as Error).message; }
  }

  async function toggleSaved() {
    if (!entry) return;
    const want = !entry.saved;
    entry = { ...entry, saved: want };
    try { await entries.toggleSaved(entry.id, want); error = null; }
    catch (e) { entry = { ...entry, saved: !want }; error = (e as Error).message; }
  }

  function viewOriginal() {
    if (entry) window.open(entry.url, '_blank', 'noopener');
  }

  function host(url: string): string {
    try { return new URL(url).host; } catch { return ''; }
  }

  function fmtDate(secs: number): string {
    return new Date(secs * 1000).toLocaleDateString(undefined, { year: 'numeric', month: 'long', day: 'numeric' });
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
      if (saveTimer !== null) clearTimeout(saveTimer);
    });
  }

  const markOnce = createMarkOnScroll({
    onMark: () => { if (entry && !entry.read) void toggleRead(); },
  });
</script>

{#snippet articleBody()}
  <div class="ts-article-source">
    <FeedAvatar feedURL={host(entry!.url)} size={14} radius={3} />
    <span class="ts-article-source-name">{host(entry!.url)}</span>
  </div>
  <h1 class="ts-article-title">{entry!.title}</h1>
  <div class="ts-article-byline">
    {#if entry!.author}<span>{entry!.author}</span><span aria-hidden="true"> · </span>{/if}
    <span>{fmtDate(entry!.published_at)}</span>
  </div>

  <div class="ts-article-actions">
    <button type="button" class="ts-article-action" onclick={toggleRead}>
      <span class="ts-action-dot" aria-hidden="true"></span>
      <span>{entry!.read ? 'Mark unread' : 'Mark read'}</span>
      <kbd class="ts-kbd">m</kbd>
    </button>
    <button type="button" class="ts-article-action" class:is-saved={entry!.saved} onclick={toggleSaved}>
      <svg width="12" height="12" viewBox="0 0 16 16" fill="currentColor" aria-hidden="true"><path d="M4 2.5h8v11l-4-3-4 3z"/></svg>
      <span>{entry!.saved ? 'Saved' : 'Save'}</span>
      <kbd class="ts-kbd">s</kbd>
    </button>
    <button type="button" class="ts-article-action" onclick={viewOriginal}>
      <svg width="12" height="12" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M9 3h4v4M13 3 7 9M11 9.5V13H3V5h3.5"/></svg>
      <span>Original</span>
      <kbd class="ts-kbd">v</kbd>
    </button>
  </div>

  <div class="ts-article-rule" aria-hidden="true">
    <span class="ts-article-rule-line"></span>
    <span class="ts-article-rule-dot"></span>
    <span class="ts-article-rule-line"></span>
  </div>

  {#if entry!.author}
    {#if useMarkOnScroll}
      <p class="ts-article-lede" {@attach markOnce}>{entry!.author}</p>
    {:else}
      <p class="ts-article-lede">{entry!.author}</p>
    {/if}
  {/if}

  <div class="ts-article-body">{@html entry!.content}</div>

  <div class="ts-article-end" aria-hidden="true">
    <span class="ts-article-rule-line"></span>
    <span class="ts-article-end-dot"></span>
    <span class="ts-article-rule-line"></span>
  </div>
  <div class="ts-article-foot">Cached locally</div>
{/snippet}

{#if $isMobile}
  <div class="ts-mobile-reader-head">
    <button type="button" class="ts-mobile-back" onclick={() => navigate('/')} aria-label="Back to Unread">
      <svg width="18" height="18" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M10 3 5 8l5 5"/></svg>
      <span>Unread</span>
    </button>
    <span class="ts-wordmark">tap<span class="ts-wordmark-dot" aria-hidden="true"></span></span>
    <button type="button" class="ts-mobile-action" class:is-saved={entry?.saved} onclick={toggleSaved} aria-label="Save">
      <svg width="18" height="18" viewBox="0 0 16 16" fill={entry?.saved ? 'currentColor' : 'none'} stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M4 2.5h8v11l-4-3-4 3z"/></svg>
    </button>
  </div>
  <div
    class="ts-mobile-reader-body"
    bind:this={scrollEl}
    onscroll={onScroll}
    data-testid="reader-scroll"
  >
    {#if error}
      <p class="err">{error}</p>
    {:else if !entry}
      <p class="loading">Loading…</p>
    {:else}
      <article class="ts-article">
        {@render articleBody()}
      </article>
    {/if}
  </div>
  <div class="ts-mobile-reader-foot">
    <button type="button" class="ts-mobile-foot-btn" onclick={toggleRead}>
      {entry?.read ? 'Mark unread' : 'Mark read'}
    </button>
    <button type="button" class="ts-mobile-foot-btn" onclick={viewOriginal}>Original</button>
    <button type="button" class="ts-mobile-foot-btn" onclick={() => navigate('/')}>Back</button>
  </div>
{:else}
  <section
    class="ts-shell ts-shell-reader measure-{measure.value}"
    class:font-sans={font.value === 'sans'}
    bind:this={scrollEl}
    onscroll={onScroll}
    data-testid="reader-scroll"
  >
    <div class="ts-backrow">
      <button type="button" class="ts-back" onclick={() => navigate('/')} aria-label="Back to Unread">
        <svg width="12" height="12" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M10 3 5 8l5 5"/></svg>
        <span>Back to Unread</span>
      </button>
      <span class="ts-back-hint"><kbd class="ts-kbd">Esc</kbd></span>
    </div>

    {#if error}
      <p class="err">{error}</p>
    {:else if !entry}
      <p class="loading">Loading…</p>
    {:else}
      <article class="ts-article">
        {@render articleBody()}
      </article>
    {/if}
  </section>
{/if}

<style>
  /* Desktop reader shell */
  .ts-shell-reader {
    max-width: 720px; margin: 0 auto;
    padding: 36px 24px 120px; overflow-y: auto; height: 100%;
  }
  .ts-shell-reader.measure-narrow .ts-article { max-width: 580px; }
  .ts-shell-reader.measure-comfortable .ts-article { max-width: 680px; }
  .ts-shell-reader.measure-wide .ts-article { max-width: 760px; }
  .ts-shell-reader.font-sans .ts-article { font-family: var(--sans); }
  .ts-shell-reader.font-sans :global(.ts-article-title),
  .ts-shell-reader.font-sans :global(.ts-article-lede),
  .ts-shell-reader.font-sans :global(.ts-article-body p),
  .ts-shell-reader.font-sans :global(.ts-article-body h2) { font-family: var(--sans); }

  .err { color: #c43a3a; font-family: var(--mono); font-size: 12px; padding: 24px; }
  .loading { color: var(--ink-3); font-family: var(--mono); font-size: 11px; padding: 24px; }

  .ts-backrow {
    display: flex; align-items: center; justify-content: space-between;
    padding: 14px 2px 0;
  }
  .ts-back {
    display: inline-flex; align-items: center; gap: 6px;
    padding: 6px 8px 6px 0;
    font-family: var(--mono); font-size: 10.5px;
    letter-spacing: 0.06em; text-transform: uppercase;
    color: var(--ink-2);
    background: transparent; border: 0; cursor: pointer; border-radius: 4px;
  }
  .ts-back:hover { color: var(--ink); }
  .ts-back-hint { font-family: var(--mono); font-size: 10px; color: var(--ink-3); }

  /* Article anatomy */
  .ts-article { margin: 0 auto; padding: 28px 0 24px; }
  .ts-article-source {
    display: flex; align-items: center; gap: 8px;
    font-family: var(--sans); font-size: 12px; color: var(--ink-2);
    margin-bottom: 22px;
  }
  .ts-article-source-name { color: var(--ink); font-weight: 500; }
  .ts-article-title {
    font-family: var(--serif); font-size: 38px; line-height: 1.12;
    font-weight: 600; color: var(--ink);
    letter-spacing: -0.02em; margin: 0 0 14px; text-wrap: balance;
  }
  .ts-article-byline {
    display: flex; flex-wrap: wrap; gap: 8px; align-items: center;
    font-family: var(--mono); font-size: 10.5px;
    letter-spacing: 0.06em; text-transform: uppercase;
    color: var(--ink-3); margin: 0 0 22px;
  }
  .ts-article-actions {
    display: flex; gap: 4px; flex-wrap: wrap;
    padding: 10px 0 4px;
    border-top: 1px solid var(--rule); border-bottom: 1px solid var(--rule);
    margin-bottom: 32px;
  }
  .ts-article-action {
    display: inline-flex; align-items: center; gap: 7px;
    padding: 8px 10px;
    font-family: var(--sans); font-size: 12.5px; font-weight: 500;
    color: var(--ink-2);
    background: transparent; border: 0; cursor: pointer; border-radius: 4px;
  }
  .ts-article-action:hover { color: var(--ink); background: var(--bg-soft); }
  .ts-article-action.is-saved { color: var(--accent); }
  .ts-action-dot {
    display: inline-block; width: 7px; height: 7px;
    border-radius: 50%; border: 1px solid currentColor; opacity: 0.7;
  }
  .ts-kbd {
    font-family: var(--mono); font-size: 10px;
    border: 1px solid var(--rule); border-bottom-width: 2px;
    border-radius: 3px; padding: 1px 5px;
    background: var(--surface); color: var(--ink-2);
  }

  .ts-article-rule {
    display: flex; align-items: center; gap: 10px;
    margin: 0 0 28px;
  }
  .ts-article-rule-line { flex: 1; height: 1px; background: var(--rule); }
  .ts-article-rule-dot {
    width: 6px; height: 6px; border-radius: 50%;
    background: var(--accent); flex-shrink: 0;
  }

  .ts-article-lede {
    font-family: var(--serif); font-size: 19px; line-height: 1.55;
    color: var(--ink); margin: 0 0 28px; font-style: italic; text-wrap: pretty;
  }
  .ts-article-body :global(p) {
    font-family: var(--serif); font-size: 17px; line-height: 1.7;
    color: var(--ink); margin: 0 0 22px; text-wrap: pretty;
  }
  .ts-article-body :global(h2) {
    font-family: var(--serif); font-size: 22px; line-height: 1.25;
    font-weight: 600; letter-spacing: -0.01em; margin: 40px 0 14px;
  }
  .ts-article-body :global(pre), .ts-article-body :global(code) {
    font-family: var(--mono); font-size: 12.5px; line-height: 1.55;
    color: var(--ink-2); background: var(--bg-soft);
    padding: 14px 16px; border-left: 2px solid var(--accent); overflow-x: auto;
  }

  .ts-article-end {
    display: flex; align-items: center; gap: 10px;
    margin: 48px 0 18px;
  }
  .ts-article-end-dot {
    width: 6px; height: 6px; border-radius: 50%;
    background: var(--ink-4); flex-shrink: 0;
  }
  .ts-article-foot {
    font-family: var(--mono); font-size: 10px;
    letter-spacing: 0.06em; text-transform: uppercase;
    color: var(--ink-3); text-align: center;
  }

  /* Mobile reader chrome */
  .ts-mobile-reader-head {
    position: fixed; top: 0; left: 0; right: 0;
    height: 56px; display: flex; align-items: center; justify-content: space-between;
    padding: 0 16px;
    background: var(--bg); border-bottom: 1px solid var(--rule);
    z-index: 100;
  }
  .ts-mobile-back {
    display: inline-flex; align-items: center; gap: 6px;
    font-family: var(--sans); font-size: 14px; color: var(--ink-2);
    background: transparent; border: 0; cursor: pointer; padding: 6px 0;
  }
  .ts-mobile-back:hover { color: var(--ink); }
  .ts-wordmark { font-family: var(--mono); font-size: 13px; font-weight: 600; color: var(--ink); }
  .ts-wordmark-dot { display: inline-block; width: 5px; height: 5px; border-radius: 50%; background: var(--accent); margin-left: 2px; vertical-align: middle; }
  .ts-mobile-action {
    background: transparent; border: 0; cursor: pointer; padding: 6px;
    color: var(--ink-2);
  }
  .ts-mobile-action.is-saved { color: var(--accent); }
  .ts-mobile-reader-body {
    padding: 72px 22px 80px; overflow-y: auto; height: 100%;
  }
  .ts-mobile-reader-foot {
    position: fixed; bottom: 0; left: 0; right: 0;
    height: 56px; display: flex; align-items: center; justify-content: space-around;
    background: var(--bg); border-top: 1px solid var(--rule);
    z-index: 100;
  }
  .ts-mobile-foot-btn {
    font-family: var(--sans); font-size: 13px; color: var(--ink-2);
    background: transparent; border: 0; cursor: pointer; padding: 8px 12px;
    border-radius: 4px;
  }
  .ts-mobile-foot-btn:hover { color: var(--ink); background: var(--bg-soft); }
</style>
