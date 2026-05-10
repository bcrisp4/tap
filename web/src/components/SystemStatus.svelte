<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { getStatus, type StatusResponse } from '../lib/status';

  let status = $state<StatusResponse | null>(null);
  let error = $state<string | null>(null);
  let interval: ReturnType<typeof setInterval>;

  async function refresh() {
    try {
      status = await getStatus();
      error = null;
    } catch (e) {
      error = e instanceof Error ? e.message : 'failed';
    }
  }

  onMount(() => {
    refresh();
    interval = setInterval(refresh, 60_000);
  });

  onDestroy(() => clearInterval(interval));

  function formatUptime(s: number): string {
    const h = Math.floor(s / 3600);
    const m = Math.floor((s % 3600) / 60);
    return h > 0 ? `${h}h ${m}m` : `${m}m`;
  }
</script>

{#if error}
  <p>Error loading status: {error}</p>
{:else if status}
  <section>
    <h3>System Status</h3>
    <dl>
      <dt>Version</dt><dd>{status.version}</dd>
      <dt>Uptime</dt><dd>{formatUptime(status.uptime_seconds)}</dd>
      <dt>Database</dt><dd>{status.db}</dd>
      <dt>Active polls</dt><dd>{status.polls_active}</dd>
    </dl>
    {#if status.recent_errors.length > 0}
      <h4>Recent errors</h4>
      <ul>
        {#each status.recent_errors as e}
          <li><time>{e.time}</time> [{e.level}] {e.event}</li>
        {/each}
      </ul>
    {:else}
      <p>No recent errors.</p>
    {/if}
  </section>
{:else}
  <p>Loading...</p>
{/if}
