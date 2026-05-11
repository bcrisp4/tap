<script lang="ts">
  import { onMount } from 'svelte';
  import SetSection from './SetSection.svelte';
  import SetRow from './SetRow.svelte';
  import { api } from '../../lib/api';
  import { formatAgo } from '../../lib/url';
  import type { Session } from '../../lib/types';

  let sessions = $state<Session[]>([]);
  let error = $state('');
  let busy = $state(false);

  onMount(async () => {
    try { sessions = await api.listSessions(); }
    catch (e) { error = e instanceof Error ? e.message : 'Could not load sessions.'; }
  });

  async function revoke(id: number) {
    busy = true;
    try {
      await api.revokeSession(id);
      sessions = sessions.filter(s => s.id !== id);
    } catch (e) {
      error = e instanceof Error ? e.message : 'Could not revoke session.';
    } finally { busy = false; }
  }

  function inferKind(ua: string): 'desktop' | 'mobile' | 'cli' {
    const u = (ua || '').toLowerCase();
    if (u.includes('iphone') || u.includes('android') || u.includes('mobile')) return 'mobile';
    if (u.includes('curl') || u.includes('cli') || !u.includes('mozilla')) return 'cli';
    return 'desktop';
  }

  function elapsed(ts: number): string {
    const secs = Math.max(0, Math.round(Date.now() / 1000 - ts));
    const s = formatAgo(secs);
    return s === '—' ? 'now' : `${s} ago`;
  }
</script>

<SetSection num="06" title="Sessions">
  <SetRow
    label="Active sessions"
    desc="Devices currently signed in to your account. Revoke anything you don't recognise."
    block
  >
    {#snippet children()}
      <div class="sess-list" role="list">
        {#each sessions as s (s.id)}
          {@const kind = inferKind(s.user_agent)}
          <div class="sess" class:is-current={s.current} role="listitem">
            <span class="sess-icon" aria-hidden="true">
              {#if kind === 'mobile'}
                <svg width="18" height="18" viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round"><rect x="6" y="2" width="8" height="16" rx="1.5"/><path d="M9.5 15.5h1"/></svg>
              {:else if kind === 'cli'}
                <svg width="18" height="18" viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round"><rect x="2" y="4" width="16" height="12" rx="1.2"/><path d="M5 8l2.5 2L5 12M10 13h4"/></svg>
              {:else}
                <svg width="18" height="18" viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round"><rect x="2" y="3" width="16" height="11" rx="1.2"/><path d="M7 17h6M10 14v3"/></svg>
              {/if}
            </span>
            <div class="sess-text">
              <div class="sess-device">
                <span>{s.user_agent || 'Unknown'}</span>
                {#if s.current}<span class="sess-tag">this session</span>{/if}
              </div>
              <div class="sess-meta">
                <span>{s.address || 'Unknown'}</span>
              </div>
            </div>
            <span class="sess-when">{elapsed(s.last_seen_at)}</span>
            <button
              type="button"
              class="sess-revoke"
              aria-label={s.current ? 'Cannot revoke current session' : `Revoke session ${s.user_agent || 'Unknown'}`}
              disabled={s.current || busy}
              onclick={() => revoke(s.id)}
            >
              <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"><path d="M4 4l8 8M12 4l-8 8"/></svg>
            </button>
          </div>
        {/each}
      </div>
      {#if error}<div role="alert" class="error">{error}</div>{/if}
    {/snippet}
  </SetRow>
</SetSection>

<style>
  .sess-list { display: flex; flex-direction: column; border: 1px solid var(--rule); border-radius: 4px; background: var(--bg); overflow: hidden; }
  .sess { display: grid; grid-template-columns: 28px minmax(0, 1fr) auto auto; align-items: center; gap: 14px; padding: 14px 16px; border-bottom: 1px solid var(--rule); }
  .sess:last-child { border-bottom: 0; }
  .sess.is-current { background: var(--accent-soft); }
  .sess-icon { width: 28px; height: 28px; display: inline-flex; align-items: center; justify-content: center; color: var(--ink-3); }
  .sess.is-current .sess-icon { color: var(--accent); }
  .sess-text { min-width: 0; }
  .sess-device { font-family: var(--sans); font-size: 13.5px; font-weight: 500; color: var(--ink); display: flex; align-items: baseline; gap: 8px; flex-wrap: wrap; }
  .sess-tag { font-family: var(--mono); font-size: 9.5px; letter-spacing: 0.08em; text-transform: uppercase; color: var(--accent); }
  .sess-meta { font-family: var(--mono); font-size: 11px; color: var(--ink-3); margin-top: 3px; }
  .sess-when { font-family: var(--mono); font-size: 11px; color: var(--ink-3); white-space: nowrap; }
  .sess-revoke { width: 28px; height: 28px; display: inline-flex; align-items: center; justify-content: center; color: var(--ink-3); background: transparent; border: 0; border-radius: 4px; cursor: pointer; }
  .sess-revoke:hover:not(:disabled) { color: #c43a3a; background: var(--bg-soft); }
  .sess-revoke:disabled { color: var(--ink-4); cursor: not-allowed; }
  .error { color: var(--ink); font-family: var(--mono); font-size: 11px; padding: 8px 0; }
</style>
