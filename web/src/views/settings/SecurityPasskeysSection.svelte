<script lang="ts">
  import { onMount } from 'svelte';
  import SetSection from './SetSection.svelte';
  import SetRow from './SetRow.svelte';
  import Button from '../../components/Button.svelte';
  import EmptyState from '../../components/EmptyState.svelte';
  import { api } from '../../lib/api';
  import type { Passkey } from '../../lib/types';
  import AddPasskeyDialog from './dialogs/AddPasskeyDialog.svelte';
  import RemovePasskeyDialog from './dialogs/RemovePasskeyDialog.svelte';

  let passkeys = $state<Passkey[]>([]);
  let addOpen = $state(false);
  let toRemove = $state<Passkey | null>(null);
  let error = $state('');

  async function load() {
    error = '';
    try { passkeys = await api.listPasskeys(); }
    catch (e) { error = e instanceof Error ? e.message : 'Could not load passkeys.'; }
  }

  onMount(load);

  function fmtDate(ts: number) { return new Date(ts * 1000).toLocaleDateString(); }
</script>

<SetSection num="05" title="Security · Passkeys" tag={passkeys.length === 0 ? 'add one' : `${passkeys.length} active`}>
  <SetRow
    label="Registered passkeys"
    desc="Sign in without a password using a device-bound credential. You can have up to 10."
    block
  >
    {#snippet children()}
      {#if passkeys.length > 0}
        <div class="pk-list">
          {#each passkeys as p (p.id)}
            <div class="pk">
              <span class="pk-icon" aria-hidden="true">
                <svg width="16" height="16" viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round">
                  <circle cx="7" cy="10" r="3.2"/><path d="M10 10h7M14 10v2.5M16.5 10v3"/>
                </svg>
              </span>
              <div class="pk-text">
                <div class="pk-label">{p.label}</div>
                <div class="pk-meta">added {fmtDate(p.created_at)}</div>
              </div>
              <button class="revoke" aria-label={`Remove passkey ${p.label}`} onclick={() => toRemove = p}>
                <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"><path d="M4 4l8 8M12 4l-8 8"/></svg>
              </button>
            </div>
          {/each}
        </div>
      {:else}
        <EmptyState title="No passkeys yet" subtitle="Add one to skip the password on this device." />
      {/if}
      <div class="actions">
        <Button variant="primary" onclick={() => addOpen = true}>Add a passkey</Button>
      </div>
      {#if error}<div role="alert" class="err">{error}</div>{/if}
    {/snippet}
  </SetRow>
</SetSection>

{#if addOpen}
  <AddPasskeyDialog onClose={() => addOpen = false} onSaved={async () => { addOpen = false; await load(); }} />
{/if}
{#if toRemove}
  <RemovePasskeyDialog
    passkeyId={toRemove.id}
    passkeyLabel={toRemove.label}
    onClose={() => toRemove = null}
    onRemoved={async () => { toRemove = null; await load(); }}
  />
{/if}

<style>
  .pk-list { display: flex; flex-direction: column; border: 1px solid var(--rule); border-radius: 4px; overflow: hidden; }
  .pk { display: grid; grid-template-columns: 24px minmax(0, 1fr) auto; align-items: center; gap: 14px; padding: 12px 14px; border-bottom: 1px solid var(--rule); background: var(--bg); }
  .pk:last-child { border-bottom: 0; }
  .pk-icon { color: var(--ink-3); display: inline-flex; }
  .pk-text { min-width: 0; }
  .pk-label { font-family: var(--sans); font-size: 13.5px; font-weight: 500; color: var(--ink); }
  .pk-meta { font-family: var(--mono); font-size: 10.5px; color: var(--ink-3); margin-top: 2px; }
  .revoke { width: 28px; height: 28px; display: inline-flex; align-items: center; justify-content: center; color: var(--ink-3); background: transparent; border: 0; border-radius: 4px; cursor: pointer; }
  .revoke:hover { color: #c43a3a; background: var(--bg-soft); }
  .actions { display: flex; gap: 8px; flex-wrap: wrap; margin-top: 14px; }
  .err { color: var(--ink); font-family: var(--mono); font-size: 11px; padding: 8px 0; }
</style>
