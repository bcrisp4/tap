<script lang="ts">
  import { api } from '../lib/api';
  import { subscriptions } from '../lib/store';
  import type { DiscoverCandidate } from '../lib/types';

  let url = $state('');
  let busy = $state(false);
  let error = $state<string | null>(null);
  let candidates = $state<DiscoverCandidate[] | null>(null);

  async function find(ev: Event) {
    ev.preventDefault();
    const trimmed = url.trim();
    if (!trimmed) return;
    busy = true; error = null; candidates = null;
    try {
      const r = await api.discoverFeeds(trimmed);
      if (r.candidates.length === 0) {
        error = 'No feeds found at that URL.';
      } else if (r.candidates.length === 1) {
        await subscriptions.add(r.candidates[0].feed_url);
        url = '';
      } else {
        candidates = r.candidates;
      }
    } catch (e) {
      error = (e as Error).message;
    } finally {
      busy = false;
    }
  }

  async function pick(c: DiscoverCandidate) {
    busy = true; error = null;
    try {
      await subscriptions.add(c.feed_url);
      url = ''; candidates = null;
    } catch (e) {
      error = (e as Error).message;
    } finally {
      busy = false;
    }
  }
</script>

<form onsubmit={find}>
  <input
    type="text"
    placeholder="Paste a feed or site URL"
    bind:value={url}
    disabled={busy}
    required
  />
  <button type="submit" disabled={busy || !url.trim()}>Find feed</button>
  {#if error}<p class="error">{error}</p>{/if}
  {#if candidates}
    <ul class="candidates" aria-label="Discovered feeds">
      {#each candidates as c (c.feed_url)}
        <li>
          <button type="button" onclick={() => pick(c)}>
            <span class="ctitle">{c.title || c.feed_url}</span>
            <span class="ctype">{c.type}</span>
          </button>
        </li>
      {/each}
    </ul>
  {/if}
</form>

<style>
  form {
    display: flex;
    flex-direction: column;
    gap: 6px;
    padding: 6px 20px 12px;
  }
  input {
    font-family: var(--sans);
    font-size: 12px;
    padding: 6px 8px;
    border: 1px solid var(--rule);
    border-radius: 4px;
    background: var(--surface);
  }
  button[type="submit"] {
    align-self: flex-start;
    font-family: var(--sans);
    font-size: 12px;
    padding: 6px 10px;
    border: 1px solid var(--rule);
    border-radius: 4px;
    background: var(--surface);
  }
  .error {
    margin: 0;
    font-family: var(--mono);
    font-size: 10px;
    color: #b14;
  }
  .candidates {
    list-style: none;
    padding: 0;
    margin: 6px 0 0;
  }
  .candidates button {
    display: flex;
    justify-content: space-between;
    width: 100%;
    padding: 6px 8px;
    border: 0;
    background: transparent;
    cursor: pointer;
    font-family: var(--sans);
    font-size: 12px;
    color: var(--ink-2);
  }
  .candidates button:hover {
    background: var(--bg-soft);
  }
  .ctype {
    font-family: var(--mono);
    font-size: 10px;
    color: var(--ink-3);
  }
</style>
