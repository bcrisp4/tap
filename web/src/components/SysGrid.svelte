<script lang="ts">
  import type { StatusResponse } from '../lib/status';

  type Props = {
    status: StatusResponse | null;
    nextPollAt: number | null;
    error?: string;
    now: number;
  };

  const { status, nextPollAt, error = '', now }: Props = $props();

  function fmtCount(n: number): string {
    return n.toLocaleString('en-US');
  }

  function relativePast(then: number | null, nowS: number): string {
    if (then == null) return '—';
    const dt = nowS - then;
    if (dt < 60) return `${Math.max(0, dt)}s ago`;
    if (dt < 3600) return `${Math.floor(dt / 60)}m ago`;
    if (dt < 86400) return `${Math.floor(dt / 3600)}h ago`;
    return `${Math.floor(dt / 86400)}d ago`;
  }

  function relativeFuture(then: number | null, nowS: number): string {
    if (then == null) return '—';
    const dt = then - nowS;
    if (dt <= 0) return 'next now';
    if (dt < 60) return `next ${dt}s`;
    if (dt < 3600) return `next ${Math.floor(dt / 60)}m`;
    if (dt < 86400) return `next ${Math.floor(dt / 3600)}h`;
    return `next ${Math.floor(dt / 86400)}d`;
  }
</script>

{#if error}
  <div class="grid-err" role="alert">{error}</div>
{:else}
  <div class="grid">
    <div class="cell">
      <span class="l">FEEDS</span>
      <span class="v" data-testid="sys-feeds-v">
        {#if !status}—{:else}{fmtCount(status.feeds_total)}{/if}
      </span>
      <span class="sub" data-testid="sys-feeds-sub">
        {#if !status}—{:else}{fmtCount(status.feeds_ok)} ok{/if}
      </span>
    </div>
    <div class="cell">
      <span class="l">ENTRIES</span>
      <span class="v" data-testid="sys-entries-v">
        {#if !status}—{:else}{fmtCount(status.entries_total)}{/if}
      </span>
      <span class="sub" data-testid="sys-entries-sub">
        {#if !status}—{:else}{fmtCount(status.entries_24h)} 24h{/if}
      </span>
    </div>
    <div class="cell">
      <span class="l">ERRORS</span>
      <span class="v" class:warn={status && status.feeds_with_errors > 0} data-testid="sys-errors-v">
        {#if !status}—{:else}{status.feeds_with_errors}{/if}
      </span>
      <span class="sub" data-testid="sys-errors-sub">
        {#if !status}—
        {:else if status.offending_feeds.length === 0}all clear
        {:else}{status.offending_feeds[0]}{#if status.offending_feeds.length > 1} +{status.offending_feeds.length - 1}{/if}
        {/if}
      </span>
    </div>
    <div class="cell">
      <span class="l">POLL</span>
      <span class="v" data-testid="sys-poll-v">
        {#if !status}—{:else}{relativePast(status.last_poll_at, now)}{/if}
      </span>
      <span class="sub" data-testid="sys-poll-sub">
        {#if !status}—{:else}{relativeFuture(nextPollAt, now)}{/if}
      </span>
    </div>
  </div>
{/if}

<style>
  .grid {
    display: grid;
    grid-template-columns: repeat(4, 1fr);
    border: 1px solid var(--rule);
    border-radius: 4px;
    overflow: hidden;
    background: var(--bg);
  }
  .cell {
    padding: 14px 16px;
    border-right: 1px solid var(--rule);
    display: flex;
    flex-direction: column;
    gap: 4px;
  }
  .cell:last-child { border-right: 0; }
  .l {
    font-family: var(--mono);
    font-size: 9.5px;
    letter-spacing: 0.12em;
    text-transform: uppercase;
    color: var(--ink-3);
  }
  .v {
    font-family: var(--mono);
    font-size: 16px;
    font-weight: 500;
    color: var(--ink);
  }
  .v.warn { color: #c97a1a; }
  :global(html.theme-dark) .v.warn { color: #f0a655; }
  .sub {
    font-family: var(--mono);
    font-size: 10px;
    color: var(--ink-3);
  }
  .grid-err {
    padding: 14px 16px;
    border: 1px solid var(--rule);
    border-radius: 4px;
    font-family: var(--mono);
    font-size: 12px;
    color: var(--ink-3);
  }
  @media (max-width: 640px) {
    .grid { grid-template-columns: repeat(2, 1fr); }
    .cell:nth-child(2) { border-right: 0; }
    .cell:nth-child(3), .cell:nth-child(4) { border-top: 1px solid var(--rule); }
  }
</style>
