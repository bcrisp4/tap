<script lang="ts">
  import { onMount } from 'svelte';
  import SetSection from './SetSection.svelte';
  import SetRow from './SetRow.svelte';
  import Segmented from '../../components/Segmented.svelte';
  import Button from '../../components/Button.svelte';
  import { poll } from '../../lib/preferences.svelte';
  import { api } from '../../lib/api';
  import { formatAgo } from '../../lib/url';

  type PollInterval = '5m' | '15m' | '1h' | 'manual';
  const options: { value: PollInterval; label: string }[] = [
    { value: '5m',     label: '5m' },
    { value: '15m',    label: '15m' },
    { value: '1h',     label: '1h' },
    { value: 'manual', label: 'Manual' },
  ];

  let lastSyncAt = $state<number | null>(null);
  let busy = $state(false);
  let error = $state('');

  onMount(async () => {
    try {
      const h = await api.health();
      lastSyncAt = h.last_poll_at ?? null;
    } catch { /* swallow — display-only */ }
  });

  async function refreshAll() {
    busy = true;
    error = '';
    try {
      const subs = await api.listSubscriptions();
      const ids = subs.map(s => s.id);
      const POOL = 4;
      const results: PromiseSettledResult<unknown>[] = [];
      for (let i = 0; i < ids.length; i += POOL) {
        const batch = ids.slice(i, i + POOL).map(id => api.refreshSubscription(id));
        results.push(...await Promise.allSettled(batch));
      }
      const failed = results.filter(r => r.status === 'rejected').length;
      if (failed > 0) {
        error = `Refreshed ${subs.length - failed} of ${subs.length} · ${failed} failed`;
      }
      try {
        const h = await api.health();
        lastSyncAt = h.last_poll_at ?? null;
      } catch { /* swallow */ }
    } catch (e) {
      error = e instanceof Error ? e.message : 'Refresh failed';
    } finally {
      busy = false;
    }
  }

  function lastSyncLabel(ts: number | null): string {
    if (!ts) return 'never';
    const secs = Math.max(0, Math.round(Date.now() / 1000 - ts));
    const s = formatAgo(secs);
    return s === '—' ? 'just now' : `${s} ago`;
  }
</script>

<SetSection num="03" title="Syncing">
  <SetRow
    label="Poll interval"
    desc="How often Tap reaches out to your feeds. Backend cadence is adaptive — this is your minimum."
  >
    {#snippet control()}
      <Segmented value={poll.interval} options={options} onChange={(v) => (poll.interval = v)} ariaLabel="Poll interval" />
    {/snippet}
  </SetRow>
  <SetRow label="Last sync" desc="Last sync {lastSyncLabel(lastSyncAt)}">
    {#snippet control()}
      <Button onclick={refreshAll} disabled={busy}>Refresh all now</Button>
    {/snippet}
  </SetRow>
  {#if error}
    <div role="alert" class="error">{error}</div>
  {/if}
</SetSection>

<style>
  .error { color: var(--ink); font-family: var(--mono); font-size: 11px; padding: 8px 0; }
</style>
