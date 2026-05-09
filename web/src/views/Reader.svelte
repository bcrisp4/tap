<script lang="ts">
  import { api } from '../lib/api';
  import { navigate } from '../lib/router';
  import type { EntryDetail } from '../lib/types';
  import FeedAvatar from '../components/FeedAvatar.svelte';
  import JunctionDot from '../components/JunctionDot.svelte';
  import { onMount } from 'svelte';

  type Props = { id: number };
  let { id }: Props = $props();

  let entry = $state<EntryDetail | null>(null);
  let error = $state<string | null>(null);

  onMount(async () => {
    try {
      entry = await api.getEntry(id);
      // Auto-mark-read on open. If the PATCH fails we leave the entry as unread
      // — the user can retry via the MARK READ button, which has the same shape.
      // NOTE (M1 security): content_html is rendered unsanitised below via {@html}.
      // HTML sanitisation is deferred to M2. Until then, do NOT expose this app
      // on a network address reachable by untrusted feed authors.
      if (entry && !entry.read) {
        try {
          await api.patchEntry(id, { read: true });
          entry = { ...entry, read: true };
        } catch { /* swallow; user can manually toggle */ }
      }
    } catch (e) {
      error = (e as Error).message;
    }
  });

  async function toggleRead() {
    if (!entry) return;
    const want = !entry.read;
    try {
      await api.patchEntry(entry.id, { read: want });
      entry = { ...entry, read: want };
    } catch (e) {
      error = (e as Error).message;
    }
  }

  function host(url: string): string {
    try { return new URL(url).host; } catch { return ''; }
  }
</script>

<div class="reader">
  <header class="header">
    <button class="back" onclick={() => navigate('/')} aria-label="Back to unread">
      ‹ <span class="back-label">UNREAD</span>
    </button>
    {#if entry}
      <div class="actions">
        <button class="action" onclick={toggleRead}>
          {entry.read ? 'MARK UNREAD' : 'MARK READ'}
        </button>
        <a class="action" href={entry.url} target="_blank" rel="noopener">VIEW ORIGINAL</a>
      </div>
    {/if}
  </header>

  <article class="body">
    {#if error}
      <p class="err">{error}</p>
    {:else if !entry}
      <p class="loading">Loading…</p>
    {:else}
      <div class="source">
        <FeedAvatar feedURL={entry.url} size={10} radius={2} />
        <span class="src-host">{host(entry.url)}</span>
      </div>
      <h1>{entry.title}</h1>
      <div class="byline">
        {#if entry.author}{entry.author}<span class="sep"> · </span>{/if}
        {new Date(entry.published_at * 1000).toLocaleDateString()}
      </div>
      <div class="divider">
        <span class="rule"></span>
        <JunctionDot color="var(--accent)" />
        <span class="rule"></span>
      </div>
      <!--
        M1 SECURITY CAVEAT: feed HTML is rendered unsanitised here.
        Sanitisation (DOMPurify or equivalent) is deferred to M2.
        Do NOT expose this app on a network reachable by untrusted feed
        authors until M2 lands. The binary defaults to 127.0.0.1:8080;
        the container binds 0.0.0.0:8080 — keep it behind Docker's
        port mapping and do not reverse-proxy it to the open internet.
      -->
      <div class="content">{@html entry.content}</div>
      <div class="divider">
        <span class="rule"></span>
        <JunctionDot color="var(--ink-4)" filled={false} />
        <span class="rule"></span>
      </div>
    {/if}
  </article>
</div>

<style>
  .reader { height: 100vh; display: flex; flex-direction: column; background: var(--bg); }
  .header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 14px 28px;
    border-bottom: 1px solid var(--rule);
    background: var(--bg);
    position: sticky; top: 0;
  }
  .back, .action {
    font-family: var(--mono);
    font-size: 11px;
    letter-spacing: 0.04em;
    text-transform: uppercase;
    color: var(--ink-2);
    padding: 6px 10px 6px 4px;
    border-radius: 4px;
  }
  .back:hover, .action:hover { background: var(--bg-soft); }
  .actions { display: flex; gap: 8px; }
  .body { max-width: 680px; margin: 0 auto; padding: 56px 56px 80px; flex: 1; overflow-y: auto; }
  .source {
    display: flex; align-items: center; gap: 8px;
    font-family: var(--sans); font-size: 12px; color: var(--ink-2);
    margin-bottom: 18px;
  }
  .src-host { font-family: var(--mono); font-size: 11px; color: var(--ink-3); }
  h1 {
    font-family: var(--serif); font-size: 38px; line-height: 1.15;
    font-weight: 600; letter-spacing: -0.015em; margin: 0 0 16px;
    text-wrap: balance;
  }
  .byline {
    font-family: var(--mono); font-size: 11px; letter-spacing: 0.04em;
    text-transform: uppercase; color: var(--ink-3); margin-bottom: 28px;
  }
  .divider {
    display: flex; align-items: center; gap: 8px;
    margin: 32px 0;
  }
  .rule {
    flex: 1; height: 1px; background: var(--rule);
  }
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
  .err { color: #b14; font-family: var(--mono); font-size: 12px; }
  .loading { color: var(--ink-3); font-family: var(--mono); font-size: 11px; }
  .sep { color: var(--ink-4); }
</style>
