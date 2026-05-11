<script lang="ts">
  import { onMount, onDestroy } from 'svelte';

  let active = $state<number | null>(null);
  let interval: ReturnType<typeof setInterval>;

  async function poll() {
    try {
      const r = await fetch('/healthz');
      if (r.ok) {
        const body = await r.json();
        active = body.polls_active ?? 0;
      }
    } catch { /* offline — leave value alone */ }
  }

  onMount(() => {
    void poll();
    interval = setInterval(poll, 30000);
  });
  onDestroy(() => clearInterval(interval));
</script>

<div class="poller-strip" role="status" aria-live="polite">
  <span class="dot" class:pulse={active !== null && active > 0}></span>
  {#if active === null}
    POLLER · waking up
  {:else if active === 0}
    POLLER · idle
  {:else}
    POLLER · {active} active
  {/if}
</div>

<style>
  .poller-strip {
    font-family: var(--mono);
    font-size: 10px;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: var(--ink-3);
    background: var(--bg-soft);
    padding: 6px 24px;
    border-bottom: 1px solid var(--rule);
    display: flex;
    align-items: center;
    gap: 8px;
  }
  .dot {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: var(--accent);
    opacity: 0.35;
  }
  .dot.pulse { animation: pulse 2.4s ease-in-out infinite; }
  @keyframes pulse {
    0%, 100% { opacity: 1; transform: scale(1); }
    50%      { opacity: 0.35; transform: scale(0.8); }
  }
</style>
