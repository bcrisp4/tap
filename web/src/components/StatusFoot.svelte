<script lang="ts">
  import { pollStatus, startPollStatus, stopPollStatus } from '../lib/pollStatus';
  import { entries } from '../lib/store';
  import { onMount, onDestroy } from 'svelte';

  onMount(() => startPollStatus());
  onDestroy(() => stopPollStatus());

  const unread = $derived($entries.items.filter(e => !e.read).length);
</script>

<footer class="foot" role="status" aria-live="polite">
  <span class="dot" aria-hidden="true"></span>
  <span>{unread === 0 ? 'all caught up' : `${unread} unread`}</span>
  <span class="sep" aria-hidden="true">·</span>
  {#if $pollStatus.active === null}
    <span>polling…</span>
  {:else if $pollStatus.active === 0}
    <span>idle</span>
  {:else}
    <span>{$pollStatus.active} polling</span>
  {/if}
  <span class="sep" aria-hidden="true">·</span>
  <span>press <kbd>?</kbd> for shortcuts</span>
</footer>

<style>
  .foot {
    display: flex; align-items: center; gap: 8px;
    padding: 24px 2px 0;
    font-family: var(--mono);
    font-size: 10px;
    letter-spacing: 0.06em;
    color: var(--ink-3);
  }
  .dot {
    width: 5px; height: 5px;
    border-radius: 50%;
    background: var(--accent);
    animation: tap-pulse 2.4s ease-in-out infinite;
    flex-shrink: 0;
  }
  .sep { color: var(--ink-4); }
  kbd {
    font-family: var(--mono);
    font-size: 10px;
    padding: 1px 5px;
    border: 1px solid var(--rule);
    border-bottom-width: 2px;
    border-radius: 3px;
    background: var(--surface);
    color: var(--ink-2);
  }
</style>
