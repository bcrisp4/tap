<script lang="ts">
  import type { StatusEvent } from '../lib/status';

  type Props = { events: StatusEvent[] };
  const { events }: Props = $props();

  function fmtTime(iso: string): string {
    const d = new Date(iso);
    const hh = String(d.getHours()).padStart(2, '0');
    const mm = String(d.getMinutes()).padStart(2, '0');
    const ss = String(d.getSeconds()).padStart(2, '0');
    return `${hh}:${mm}:${ss}`;
  }
</script>

{#if events.length === 0}
  <div class="empty">No recent events</div>
{:else}
  <div class="table">
    {#each events as e (e.time + e.event)}
      <div class="err level-{e.level}">
        <span class="t" data-testid="err-time">{fmtTime(e.time)}</span>
        <span class="lvl lvl-{e.level}">{e.level}</span>
        <span class="m">{e.event}</span>
      </div>
    {/each}
  </div>
{/if}

<style>
  .table {
    border: 1px solid var(--rule);
    border-radius: 4px;
    overflow: hidden;
    background: var(--bg);
  }
  .err {
    display: grid;
    grid-template-columns: 76px 58px 1fr;
    gap: 14px;
    align-items: center;
    padding: 9px 14px;
    border-bottom: 1px solid var(--rule);
    font-family: var(--mono);
    font-size: 11.5px;
    color: var(--ink-2);
  }
  .err:last-child { border-bottom: 0; }
  .t { color: var(--ink-3); }
  .m {
    color: var(--ink);
    white-space: nowrap; overflow: hidden; text-overflow: ellipsis;
  }
  .lvl {
    font-family: var(--mono);
    font-size: 9.5px;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    padding: 2px 6px;
    border-radius: 2px;
    text-align: center;
    border: 1px solid var(--rule);
    color: var(--ink-3);
    background: var(--bg);
  }
  .lvl-error { color: #c43a3a; border-color: rgba(196, 58, 58, 0.45); background: rgba(196, 58, 58, 0.06); }
  :global(html.theme-dark) .lvl-error { color: #ec7a7a; border-color: rgba(236, 122, 122, 0.45); background: rgba(236, 122, 122, 0.08); }
  .lvl-warn  { color: #c97a1a; border-color: rgba(201, 122, 26, 0.45); background: rgba(201, 122, 26, 0.06); }
  :global(html.theme-dark) .lvl-warn { color: #f0a655; border-color: rgba(240, 166, 85, 0.45); background: rgba(240, 166, 85, 0.08); }
  .level-error { background: rgba(196, 58, 58, 0.02); }
  :global(html.theme-dark) .level-error { background: rgba(236, 122, 122, 0.04); }
  .empty {
    padding: 16px;
    text-align: center;
    color: var(--ink-3);
    font-family: var(--mono);
    font-size: 11.5px;
    border: 1px solid var(--rule);
    border-radius: 4px;
  }
</style>
