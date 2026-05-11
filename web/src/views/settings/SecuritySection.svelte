<script lang="ts">
  import { onMount } from 'svelte';
  import SetSection from './SetSection.svelte';
  import SetRow from './SetRow.svelte';
  import Button from '../../components/Button.svelte';
  import EmptyState from '../../components/EmptyState.svelte';
  import { auth } from '../../lib/auth';
  import { api } from '../../lib/api';
  import type { Passkey } from '../../lib/types';
  import EnrolTOTPDialog from './dialogs/EnrolTOTPDialog.svelte';
  import DisableTOTPDialog from './dialogs/DisableTOTPDialog.svelte';
  import RegenerateCodesDialog from './dialogs/RegenerateCodesDialog.svelte';
  import ViewRecoveryCodesDialog from './dialogs/ViewRecoveryCodesDialog.svelte';
  import AddPasskeyDialog from './dialogs/AddPasskeyDialog.svelte';
  import RemovePasskeyDialog from './dialogs/RemovePasskeyDialog.svelte';

  type TOTPOpen = null | 'enrol' | 'disable' | 'regen' | 'view';
  let totpOpen = $state<TOTPOpen>(null);
  let viewCodes = $state<string[]>([]);
  let regenerated = $state(false);

  const hasTOTP = $derived($auth.user?.has_totp ?? false);

  let passkeys = $state<Passkey[]>([]);
  let addOpen = $state(false);
  let toRemove = $state<Passkey | null>(null);
  let pkError = $state('');

  const webAuthnSupported = typeof window !== 'undefined' &&
    !!window.PublicKeyCredential &&
    !!navigator.credentials;

  async function loadPasskeys() {
    pkError = '';
    try { passkeys = await api.listPasskeys(); }
    catch (e) { pkError = e instanceof Error ? e.message : 'Could not load passkeys.'; }
  }

  onMount(loadPasskeys);

  function fmtDate(ts: number) { return new Date(ts * 1000).toLocaleDateString(); }
</script>

<SetSection num="05" title="Security">
  <div class="sub-group">
    <div class="sub-heading">Two-factor authentication</div>
    <SetRow
      label="Authenticator app (TOTP)"
      desc="Pair Tap with an authenticator app. Required on new sign-ins once enabled."
      block
    >
      {#snippet children()}
        <div class="status" class:on={hasTOTP}>
          <span class="dot" aria-hidden="true"></span>
          {#if hasTOTP}
            <span class="strong">Enabled</span>
            <span class="sep" aria-hidden="true"></span>
            <span>SHA-1 · 30s · 6 digits</span>
          {:else}
            <span class="strong">Not enrolled</span>
            <span class="sep" aria-hidden="true"></span>
            <span>your account relies on password only</span>
          {/if}
        </div>
        <div class="actions">
          {#if hasTOTP}
            <Button variant="accent" onclick={() => { regenerated = true; totpOpen = 'regen'; }}>Regenerate recovery codes</Button>
            <Button variant="danger" onclick={() => totpOpen = 'disable'}>Disable TOTP</Button>
          {:else}
            <Button variant="primary" onclick={() => totpOpen = 'enrol'}>Set up authenticator</Button>
          {/if}
        </div>
      {/snippet}
    </SetRow>
  </div>

  <div class="sub-group">
    <div class="sub-heading">Passkeys</div>
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
        {#if webAuthnSupported}
          <div class="actions">
            <Button variant="primary" onclick={() => addOpen = true}>Add a passkey</Button>
          </div>
        {:else}
          <p class="no-webauthn">Your browser doesn't support passkeys.</p>
        {/if}
        {#if pkError}<div role="alert" class="err">{pkError}</div>{/if}
      {/snippet}
    </SetRow>
  </div>
</SetSection>

{#if totpOpen === 'enrol'}
  <EnrolTOTPDialog
    onClose={() => totpOpen = null}
    onSuccess={(codes) => { viewCodes = codes; regenerated = false; totpOpen = 'view'; }}
  />
{:else if totpOpen === 'disable'}
  <DisableTOTPDialog onClose={() => totpOpen = null} onSuccess={() => totpOpen = null} />
{:else if totpOpen === 'regen'}
  <RegenerateCodesDialog
    onClose={() => totpOpen = null}
    onSuccess={(codes) => { viewCodes = codes; totpOpen = 'view'; }}
  />
{:else if totpOpen === 'view'}
  <ViewRecoveryCodesDialog codes={viewCodes} {regenerated} onClose={() => { totpOpen = null; viewCodes = []; }} />
{/if}

{#if addOpen}
  <AddPasskeyDialog onClose={() => addOpen = false} onSaved={async () => { addOpen = false; await loadPasskeys(); }} />
{/if}
{#if toRemove}
  <RemovePasskeyDialog
    passkeyId={toRemove.id}
    passkeyLabel={toRemove.label}
    onClose={() => toRemove = null}
    onRemoved={async () => { toRemove = null; await loadPasskeys(); }}
  />
{/if}

<style>
  .sub-group { padding-top: 8px; }
  .sub-heading {
    font-family: var(--mono); font-size: 10px; letter-spacing: 0.12em;
    text-transform: uppercase; color: var(--ink-4);
    padding: 12px 0 4px;
  }
  .status { display: flex; align-items: center; gap: 12px; padding: 10px 14px; border: 1px solid var(--rule); background: var(--bg-soft); border-radius: 4px; font-family: var(--mono); font-size: 11px; color: var(--ink-2); letter-spacing: 0.02em; }
  .status .dot { width: 6px; height: 6px; border-radius: 50%; background: var(--ink-4); }
  .status.on .dot { background: var(--accent); }
  .status .sep { width: 3px; height: 3px; border-radius: 50%; background: var(--ink-4); }
  .strong { color: var(--ink); font-weight: 500; }
  .actions { display: flex; gap: 8px; flex-wrap: wrap; margin-top: 14px; }
  .pk-list { display: flex; flex-direction: column; border: 1px solid var(--rule); border-radius: 4px; overflow: hidden; }
  .pk { display: grid; grid-template-columns: 24px minmax(0, 1fr) auto; align-items: center; gap: 14px; padding: 12px 14px; border-bottom: 1px solid var(--rule); background: var(--bg); }
  .pk:last-child { border-bottom: 0; }
  .pk-icon { color: var(--ink-3); display: inline-flex; }
  .pk-text { min-width: 0; }
  .pk-label { font-family: var(--sans); font-size: 13.5px; font-weight: 500; color: var(--ink); }
  .pk-meta { font-family: var(--mono); font-size: 10.5px; color: var(--ink-3); margin-top: 2px; }
  .revoke { width: 28px; height: 28px; display: inline-flex; align-items: center; justify-content: center; color: var(--ink-3); background: transparent; border: 0; border-radius: 4px; cursor: pointer; }
  .revoke:hover { color: #c43a3a; background: var(--bg-soft); }
  .no-webauthn { font-family: var(--mono); font-size: 11px; color: var(--ink-3); margin-top: 10px; }
  .err { color: var(--ink); font-family: var(--mono); font-size: 11px; padding: 8px 0; }
</style>
